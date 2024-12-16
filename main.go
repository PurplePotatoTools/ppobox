package main

import (
	"log"
	"os"
)

const description = `
easy-sshd: Start an SSH server that allows you to login with a password.
gotty: Share your terminal as a web application.
`

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Invalid number of arguments")
	}
	var op string = os.Args[1]

	switch op {
	case "easy-sshd":
		easySSHD()
	case "gosftp-push":
		sftpPush()
	case "gotty":
		goTTY()
	case "goftp":
		goftpMain()
	case "goftp-push":
		goftpPush()
	case "file-summary":
		fileSummaryMain()
	case "file-summary-diff":
		fileSummaryDiffMain()
	case "file-summary-web":
		fileSummaryWebServer()
	default:
		panic("Invalid operation")
	}
}
