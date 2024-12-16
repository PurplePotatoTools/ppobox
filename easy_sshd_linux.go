package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

// SftpHandler handler for SFTP subsystem
func SftpHandler(sess ssh.Session) {
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Printf("sftp server init error: %s\n", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		fmt.Println("sftp client exited session.")
	} else if err != nil {
		fmt.Println("sftp server completed with error:", err)
	}
}

func startSSHD(host, port, user, password, command string) {
	ssh.Handle(func(s ssh.Session) {
		cmd := exec.Command("sh", "-c", command)
		ptyReq, winCh, isPty := s.Pty()
		if isPty {
			cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
			f, err := pty.Start(cmd)
			if err != nil {
				panic(err)
			}
			go func() {
				for win := range winCh {
					setWinsize(f, win.Width, win.Height)
				}
			}()
			go func() {
				io.Copy(f, s) // stdin
			}()
			io.Copy(s, f) // stdout
			cmd.Wait()
		} else {
			io.WriteString(s, "No PTY requested.\n")
			s.Exit(1)
		}
	})

	log.Printf("Starting ssh server on %s:%s", host, port)
	sshServer := ssh.Server{
		Addr: host + ":" + port,
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": SftpHandler,
		},
		PasswordHandler: func(ctx ssh.Context, pass string) bool {
			return ctx.User() == user && pass == password
		},
	}
	log.Fatal(sshServer.ListenAndServe())
}

func easySSHD() {

	var port string
	var host string
	var user string
	var password string
	var command string

	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)

	newFlag.StringVar(&port, "port", "2222", "Port to listen on")
	newFlag.StringVar(&host, "host", "0.0.0.0", "Host to listen on")
	newFlag.StringVar(&user, "user", "root", "User to login")
	newFlag.StringVar(&password, "password", "root", "Password to login")
	newFlag.StringVar(&command, "command", "/bin/bash --login", "Command to execute for shell")

	log.Println("Starting easy-sshd args : ", os.Args[1:])
	err := newFlag.Parse(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}

	startSSHD(host, port, user, password, command)
}
