package main

import (
	"flag"
	"io"
	"os"

	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh"
)

func sftpPush() {

	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var username, password, host, port, localPath, remotePath string

	newFlag.StringVar(&username, "username", "", "Username")
	newFlag.StringVar(&password, "password", "", "Password")
	newFlag.StringVar(&host, "host", "127.0.0.1", "Host")
	newFlag.StringVar(&port, "port", "2222", "Port")
	newFlag.StringVar(&localPath, "local", "", "Local path")
	newFlag.StringVar(&remotePath, "remote", "", "Remote path")

	err := newFlag.Parse(os.Args[2:])

	if err != nil {
		panic(err)
	}

	conn, err := ssh.Dial("tcp", host+":"+port, &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})

	if err != nil {
		panic(err)
	}

	client, err := sftp.NewClient(conn)

	if err != nil {
		panic(err)
	}

	defer client.Close()

	f, err := os.Open(localPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fileInfo, err := f.Stat()
	// bar := progressbar.Default(fileInfo.Size(), "uploading")

	bar := progressbar.DefaultBytes(
		fileInfo.Size(),
		"uploading",
	)

	remoteFile, err := client.Create(remotePath)
	if err != nil {
		panic(err)
	}

	defer remoteFile.Close()

	_, err = io.Copy(io.MultiWriter(remoteFile, bar), f)

	if err != nil {
		panic(err)
	}

}
