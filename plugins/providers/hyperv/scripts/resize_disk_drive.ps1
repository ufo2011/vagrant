# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

#Requires -Modules VagrantMessages

param(
    [Parameter(Mandatory=$true)]
    [string]$VmId,
    [Parameter(Mandatory=$true)]
    [string]$DiskFilePath,
    [Parameter(Mandatory=$true)]
    [UInt64]$DiskSize
)

try {
    $VM = Hyper-V\Get-VM -Id $VmId
    Hyper-V\Resize-VHD -Path $DiskFilePath -SizeBytes $DiskSize
} catch {
    Write-ErrorMessage "Failed to resize disk ${DiskFilePath} for VM ${VM}: ${PSItem}"
    exit 1
}
