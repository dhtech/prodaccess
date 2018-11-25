package main

const (
	psPurge = `
Get-ChildItem -Path cert:\CurrentUser\My -Recurse -EKU "*Client Authentication*" -ExpiringInDays 0 | Remove-Item
`
	psImport = `
Import-PfxCertificate -FilePath %s -CertStoreLocation Cert:\CurrentUser\My
`
)
