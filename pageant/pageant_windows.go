//
// Copyright (c) 2014 David Mzareulyan
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software
// and associated documentation files (the "Software"), to deal in the Software without restriction,
// including without limitation the rights to use, copy, modify, merge, publish, distribute,
// sublicense, and/or sell copies of the Software, and to permit persons to whom the Software
// is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial
// portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING
// BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
//

// +build windows

package pageant

// see https://github.com/Yasushi/putty/blob/master/windows/winpgntc.c#L155
// see https://github.com/paramiko/paramiko/blob/master/paramiko/win_pageant.py

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	. "syscall"
	"unsafe"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Maximum size of message can be sent to pageant
const MaxMessageLen = 8192

var (
	ErrPageantNotFound = errors.New("pageant process not found")
	ErrSendMessage     = errors.New("error sending message")

	ErrMessageTooLong       = errors.New("message too long")
	ErrInvalidMessageFormat = errors.New("invalid message format")
	ErrResponseTooLong      = errors.New("response too long")
)

const (
	agentAddIdentity = 17
	agentCopydataID  = 0x804e50ba
	agentSuccess     = 6
	wmCopydata       = 74
)

type copyData struct {
	dwData uintptr
	cbData uint32
	lpData unsafe.Pointer
}

var (
	lock sync.Mutex

	winFindWindow         = winAPI("user32.dll", "FindWindowW")
	winGetCurrentThreadID = winAPI("kernel32.dll", "GetCurrentThreadId")
	winSendMessage        = winAPI("user32.dll", "SendMessageW")
)

func winAPI(dllName, funcName string) func(...uintptr) (uintptr, uintptr, error) {
	proc := MustLoadDLL(dllName).MustFindProc(funcName)
	return func(a ...uintptr) (uintptr, uintptr, error) { return proc.Call(a...) }
}

// Available returns true if Pageant is running
func Available() bool { return pageantWindow() != 0 }

// Query sends message msg to Pageant and returns response or error.
// 'msg' is raw agent request with length prefix
// Response is raw agent response with length prefix
func query(msg []byte) ([]byte, error) {
	if len(msg) > MaxMessageLen {
		return nil, ErrMessageTooLong
	}

	msgLen := binary.BigEndian.Uint32(msg[:4])
	if len(msg) != int(msgLen)+4 {
		return nil, ErrInvalidMessageFormat
	}

	lock.Lock()
	defer lock.Unlock()

	paWin := pageantWindow()

	if paWin == 0 {
		return nil, ErrPageantNotFound
	}

	thID, _, _ := winGetCurrentThreadID()
	mapName := fmt.Sprintf("PageantRequest%08x", thID)
	pMapName, _ := UTF16PtrFromString(mapName)

	mmap, err := CreateFileMapping(InvalidHandle, nil, PAGE_READWRITE, 0, MaxMessageLen+4, pMapName)
	if err != nil {
		return nil, err
	}
	defer CloseHandle(mmap)

	ptr, err := MapViewOfFile(mmap, FILE_MAP_WRITE, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	defer UnmapViewOfFile(ptr)

	mmSlice := (*(*[MaxMessageLen]byte)(unsafe.Pointer(ptr)))[:]

	copy(mmSlice, msg)

	mapNameBytesZ := append([]byte(mapName), 0)

	cds := copyData{
		dwData: agentCopydataID,
		cbData: uint32(len(mapNameBytesZ)),
		lpData: unsafe.Pointer(&(mapNameBytesZ[0])),
	}

	resp, _, _ := winSendMessage(paWin, wmCopydata, 0, uintptr(unsafe.Pointer(&cds)))

	if resp == 0 {
		return nil, ErrSendMessage
	}

	respLen := binary.BigEndian.Uint32(mmSlice[:4])
	if respLen > MaxMessageLen-4 {
		return nil, ErrResponseTooLong
	}

	respData := make([]byte, respLen+4)
	copy(respData, mmSlice)

	return respData, nil
}

func pageantWindow() uintptr {
	nameP, _ := UTF16PtrFromString("Pageant")
	h, _, _ := winFindWindow(uintptr(unsafe.Pointer(nameP)), uintptr(unsafe.Pointer(nameP)))
	return h
}

type PageantAgent struct {
	c *conn
	agent.Agent
}

// New returns new ssh-agent instance (see http://golang.org/x/crypto/ssh/agent)
func New() *PageantAgent {
	c := &conn{}
	return &PageantAgent{c, agent.NewClient(c)}
}

type conn struct {
	sync.Mutex
	buf []byte
}

func (c *conn) Write(p []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	resp, err := query(p)
	if err != nil {
		return 0, err
	}
	c.buf = append(c.buf, resp...)
	return len(p), nil
}

func (c *conn) Read(p []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	if len(c.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

type loadHackCertificateMsg struct {
	Type        string `sshtype:"17"`
	Keyblob     []byte
	Comment     string
}

func (p *PageantAgent) LoadHackCertificate(c string) error {
	// TODO(bluecmd): Since the PuTTY maintainers doesn't want to add
	// support for user certificates right now, we have hacked in support
	// in our own Pageant. The way to load certificates into that version
	// is not compatible with the OpenSSH wire format, so we do it ad-hoc here.
	// The format is:
	// - Length (BE 4 byte)
	// - Message type (SSH2_AGENTC_ADD_IDENTITY, 17, 1 byte)
	// - Algorithm name length (BE 4 byte)
	// - Algorithm name ("blablaba-cert-v01@openssh.com", string)
	// - Keyblob (The classical "middle field" of the openssh keys)
	// - Comment length (BE 4 byte)
	// - Comment ("my comment", string)

	parts := strings.SplitN(c, " ", 3)
	t := parts[0]
	blob := parts[1]
	comment := ""
	if len(parts) > 2 {
		comment = parts[2]
	}

	bblob, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return fmt.Errorf("base64 decode error on certificate: %v", err)
	}

	req := ssh.Marshal(loadHackCertificateMsg{Type: t, Keyblob: bblob, Comment: comment})
	msg := make([]byte, 4 + len(req))
	binary.BigEndian.PutUint32(msg, uint32(len(req)))
	copy(msg[4:], req)
	p.c.Write(msg)

	// The reply is always (AFAIK) 1 byte status code, so ignore everything else.
	resp := make([]byte, 5)
	n, err := p.c.Read(resp)
	if err != nil {
		return err
	}

	if n != 5 || resp[4] != agentSuccess {
		return fmt.Errorf("Pageant returned error %v", resp[4])
	}
	return nil
}
