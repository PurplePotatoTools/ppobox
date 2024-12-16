package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/adler32"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

type HashTypeImpl string

const (
	HashTypeMD5     HashTypeImpl = "md5"
	HashTypeAdler32 HashTypeImpl = "adler32"
	HashTypeNone    HashTypeImpl = ""
)

type FileSummary struct {
	DirName  string
	FileName string
	Hash     string
	HashType HashTypeImpl
	FileSize int64
	IsExist  bool
	IsDir    bool
	IsLink   bool
	IsSkip   bool
}

func NewFileSummary(dirName, fileName string, hashType HashTypeImpl) *FileSummary {
	resp := &FileSummary{DirName: dirName, FileName: fileName, HashType: hashType}
	return resp
}

func FileSummaryEqual(fs, other FileSummary) bool {

	if fs.Hash != other.Hash {
		return false
	}
	if fs.FileSize != other.FileSize {
		return false
	}
	if fs.IsDir != other.IsDir {
		return false
	}
	if fs.IsLink != other.IsLink {
		return false
	}
	if fs.IsExist != other.IsExist {
		return false
	}

	return true
}
func (fs *FileSummary) Load() error {
	fullFileName := filepath.Join(fs.DirName, fs.FileName)
	// Check if the file exists
	fileStat, err := os.Stat(fullFileName)
	if err != nil {
		if os.IsNotExist(err) {
			fs.IsExist = false
			return nil
		} else {
			fs.IsSkip = true
			return nil
		}
	}

	fs.IsExist = true

	fs.IsDir = fileStat.IsDir()

	// Check if the file is a link
	if fileStat.Mode()&os.ModeSymlink != 0 {
		fs.IsLink = true
		return nil
	}
	if fs.IsDir {
		return nil
	}

	fs.FileSize = fileStat.Size()

	// Calculate the hash

	switch fs.HashType {
	case HashTypeMD5:
		fs.Hash, err = calculateMD5(fullFileName)
	case HashTypeAdler32:
		fs.Hash, err = calculateAdler32(fullFileName)
	default:
		fs.Hash = ""
	}
	return nil
}

func calculateMD5(fileName string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		panic(err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func calculateAdler32(fileName string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	hash := adler32.New()
	if _, err := io.Copy(hash, file); err != nil {
		panic(err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

const (
	Size2MB   = 2 * 1024 * 1024
	Size10MB  = Size2MB * 5
	Size50MB  = Size10MB * 5
	Size100MB = Size10MB * 10
	Size500MB = Size100MB * 5
)

func getDirSummary(dirName string, bigFileThreshold int64) (resp []FileSummary, err error) {
	walker := func(fileName string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fileSize := fi.Size()
		hashType := HashTypeAdler32
		if fileSize > bigFileThreshold {
			hashType = HashTypeNone
		}
		relativePath, _ := filepath.Rel(dirName, fileName)
		relativePath = filepath.ToSlash(relativePath)
		if relativePath == "." {
			return nil
		}

		fs := NewFileSummary(dirName, relativePath, hashType)
		if err := fs.Load(); err != nil {
			return err
		}

		resp = append(resp, *fs)
		return nil
	}

	if err := filepath.Walk(dirName, walker); err != nil {
		return nil, err
	}

	// 按文件名排序
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].FileName < resp[j].FileName
	})

	return resp, nil

}

func contrastFileSummaryMove(src, target []FileSummary) (resp []string) {
	// 只要不是完全一致，就认为是不一致的, 对比后获取两个切片的所有diff相关文件

	srcMap := make(map[string]FileSummary)
	for _, fs := range src {
		srcMap[fs.FileName] = fs
	}

	targetMap := make(map[string]FileSummary)
	for _, fs := range target {
		targetMap[fs.FileName] = fs

		if _, ok := srcMap[fs.FileName]; !ok {
			resp = append(resp, fs.FileName)
		}

		if !FileSummaryEqual(srcMap[fs.FileName], fs) {
			resp = append(resp, fs.FileName)
		}
	}

	for _, fs := range src {
		if _, ok := targetMap[fs.FileName]; !ok {
			resp = append(resp, fs.FileName)
		}

		if !FileSummaryEqual(targetMap[fs.FileName], fs) {
			resp = append(resp, fs.FileName)
		}
	}

	// 去重
	uniqueMap := make(map[string]bool)
	for _, v := range resp {
		uniqueMap[v] = true
	}

	resp = []string{}
	for k := range uniqueMap {
		resp = append(resp, k)
	}

	// 按文件名排序
	sort.Strings(resp)

	return resp
}
func fileSummaryDiffMain() {
	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var srcInfoFile string
	var targetInfoFile string
	newFlag.StringVar(&srcInfoFile, "src", "", "Source file summary")
	newFlag.StringVar(&targetInfoFile, "target", "", "Target file summary")
	err := newFlag.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	srcFile, err := os.ReadFile(srcInfoFile)
	if err != nil {
		panic(err)
	}

	targetFile, err := os.ReadFile(targetInfoFile)
	if err != nil {
		panic(err)
	}

	var src []FileSummary
	err = json.Unmarshal(srcFile, &src)
	if err != nil {
		panic(err)
	}

	var target []FileSummary
	err = json.Unmarshal(targetFile, &target)
	if err != nil {
		panic(err)
	}

	resp := contrastFileSummaryMove(src, target)
	respJSON, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(respJSON))
}
func fileSummaryMain() {
	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var dirName string
	var bigFileThreshold int64
	var outputFileName string
	newFlag.StringVar(&dirName, "dir", ".", "Directory to summarize")
	newFlag.Int64Var(&bigFileThreshold, "bigfile", Size10MB, "Threshold for big files. default: 10MB")
	newFlag.StringVar(&outputFileName, "output", "", "Output file name")

	err := newFlag.Parse(os.Args[2:])

	if err != nil {
		panic(err)
	}
	resp, err := getDirSummary(dirName, bigFileThreshold)
	if err != nil {
		panic(err)

	}
	respJSON, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		panic(err)
	}
	if outputFileName != "" {
		err = os.WriteFile(outputFileName, respJSON, 0644)
		if err != nil {
			panic(err)
		}
	} else {

		fmt.Println(string(respJSON))
	}
}

func fileSummaryWebServer() {
	newFlag := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var port string
	var ftpPort string
	var sftpPort string
	var dirName string
	var bigFileThreshold int64
	newFlag.StringVar(&dirName, "dir", ".", "Directory to summarize")
	newFlag.Int64Var(&bigFileThreshold, "bigfile", Size10MB, "Threshold for big files. default: 10MB")
	newFlag.StringVar(&port, "port", "8080", "Port to listen on")
	newFlag.StringVar(&ftpPort, "ftp-port", "", "FTP Port to listen on")
	newFlag.StringVar(&sftpPort, "sftp-port", "", "SFTP Port to listen on")
	err := newFlag.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}
	var logBytesBuffer = bytes.NewBuffer(nil)

	http.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		// write the log to the response
		logBytesBuffer.Write([]byte("end;\n"))
		w.Write([]byte(logBytesBuffer.String()))
	})

	http.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		os.Exit(0)
	})

	http.HandleFunc("/file-summary-diff", func(w http.ResponseWriter, r *http.Request) {
		src, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var srcFS []FileSummary
		err = json.Unmarshal(src, &srcFS)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		target, err := getDirSummary(dirName, bigFileThreshold)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := contrastFileSummaryMove(srcFS, target)
		respJSON, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(respJSON)
	})

	http.HandleFunc("/file-delete", func(w http.ResponseWriter, r *http.Request) {
		fileName := r.FormValue("filename")
		if fileName == "" {
			http.Error(w, "filename is required", http.StatusBadRequest)
			return
		}

		filePath := filepath.Join(dirName, fileName)
		err := os.RemoveAll(filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("ok"))
	})

	http.HandleFunc("/file-upload", func(w http.ResponseWriter, r *http.Request) {

		body := map[string]string{}

		err := json.NewDecoder(r.Body).Decode(&body)

		if err != nil {
			http.Error(w, fmt.Sprintf("decode body failed: %v", err), http.StatusBadRequest)
			return
		}

		fileName := body["filename"]
		if fileName == "" {
			http.Error(w, "filename is required", http.StatusBadRequest)
			return
		}

		fileBase64 := body["file"]
		if fileBase64 == "" {
			http.Error(w, "file is required", http.StatusBadRequest)
			return
		}
		fileContent, err := base64.StdEncoding.DecodeString(fileBase64)
		if err != nil {
			http.Error(w, fmt.Sprintf("decode file failed: %v", err), http.StatusBadRequest)
			return
		}

		filePath := filepath.Join(dirName, fileName)
		log.Println("upload file:", filePath)
		logBytesBuffer.WriteString(fmt.Sprintf("upload file: %v\n", filePath))
		dirPath := filepath.Dir(filePath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, 0755)
			if err != nil {
				log.Println("create dir failed:", err)
				logBytesBuffer.WriteString(fmt.Sprintf("create dir failed: %v\n", err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		os.RemoveAll(filePath)
		err = os.WriteFile(filePath, fileContent, 0644)
		if err != nil {
			http.Error(w, fmt.Sprintf("write file failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("ok"))
	})

	if ftpPort != "" {
		go func() {
			log.Println("FTP server is starting")
			goftpStart(
				"",
				"",
				"127.0.0.1",
				ftpPort,
				dirName,
			)
		}()
	}

	if sftpPort != "" {
		go func() {
			log.Println("SFTP server is starting")
			startSSHD(
				"127.0.0.1",
				sftpPort,
				"root",
				"root",
				"/bin/bash --login",
			)
		}()
	}

	// run the server
	http.ListenAndServe("127.0.0.1:"+port, nil)
}
