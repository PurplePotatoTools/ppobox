package main

import (
	"flag"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cast"
	"goftp.io/server/v2"
	ftpfile "goftp.io/server/v2/driver/file"
)

type AnyAuth struct {
}

func (a *AnyAuth) CheckPasswd(ctx *server.Context, user, pass string) (bool, error) {
	return true, nil
}

type MyDriver struct {
	server.Driver
	RootPath string
}

func (driver *MyDriver) realPath(path string) string {
	paths := strings.Split(path, "/")
	return filepath.Join(append([]string{driver.RootPath}, paths...)...)
}

func (driver *MyDriver) ListDir(ctx *server.Context, path string, callback func(os.FileInfo) error) error {
	basepath := driver.realPath(path)
	return filepath.Walk(basepath, func(f string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return nil
		}
		rPath, _ := filepath.Rel(basepath, f)
		if rPath == info.Name() {
			err = callback(info)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
}

func goftpStart(username, password, host, port, rootPath string) {
	driver, err := ftpfile.NewDriver(rootPath)
	if err != nil {
		panic(err)
	}
	myDriver := &MyDriver{Driver: driver, RootPath: rootPath}

	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	currentOwner := u.Username
	currentGroup := u.Username

	var auth server.Auth
	auth = &AnyAuth{}
	if username != "" && password != "" {
		auth = &server.SimpleAuth{
			Name:     username,
			Password: password,
		}
	}
	opt := &server.Options{
		Name:     "go",
		Port:     cast.ToInt(port),
		Hostname: host,
		Auth:     auth,
		Perm: server.NewSimplePerm(
			currentOwner,
			currentGroup,
		),
		Driver: myDriver,
	}

	server, err := server.NewServer(opt)
	if err != nil {
		panic(err)
	}
	server.ListenAndServe()
}

func goftpMain() {
	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var username, password, host, port, rootPath string
	newFlag.StringVar(&username, "username", "", "Username")
	newFlag.StringVar(&password, "password", "", "Password")
	newFlag.StringVar(&host, "host", "127.0.0.1", "Host")
	newFlag.StringVar(&port, "port", "2121", "Port")
	newFlag.StringVar(&rootPath, "root", ".", "Root path")
	err := newFlag.Parse(os.Args[2:])

	if err != nil {
		panic(err)
	}

	goftpStart(username, password, host, port, rootPath)
}

type readerFile struct {
	*os.File
	bar *progressbar.ProgressBar
}

func (f *readerFile) Read(p []byte) (n int, err error) {
	n, err = f.File.Read(p)
	f.bar.Add(n)
	return
}

func goftpPush() {

	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var username, password, host, port, localPath, remotePath string

	newFlag.StringVar(&username, "username", "", "Username")
	newFlag.StringVar(&password, "password", "", "Password")
	newFlag.StringVar(&host, "host", "127.0.0.1", "Host")
	newFlag.StringVar(&port, "port", "2121", "Port")
	newFlag.StringVar(&localPath, "local", "", "Local path")
	newFlag.StringVar(&remotePath, "remote", "", "Remote path")

	err := newFlag.Parse(os.Args[2:])

	if err != nil {
		panic(err)
	}

	c, err := ftp.Dial(host+":"+port, ftp.DialWithTimeout(15*time.Second))
	if err != nil {
		panic(err)
	}

	err = c.Login(username, password)
	if err != nil {
		panic(err)
	}

	err = c.Type(ftp.TransferTypeBinary)
	if err != nil {
		panic(err)
	}

	f, err := os.Open(localPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fileInfo, err := f.Stat()
	bar := progressbar.Default(fileInfo.Size(), "uploading")
	err = c.Stor(remotePath, &readerFile{
		File: f,
		bar:  bar,
	})

	if err := c.Quit(); err != nil {
		panic(err)
	}

}
