package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	basedir := "./staging"
	withPrefix := "vptech/data/contract"
	newPrefix := "travel/tracking/datalake_exporter/pkg/proto"

	if len(os.Args) >= 2 {
		basedir = os.Args[1]
	}
	CreateProtoFiles(basedir, ".proto")
	PatchFiles(withPrefix, newPrefix, basedir, ".go")
}

func NewResultWriter(filename string) *ResultWriter {
	return &ResultWriter{
		fileName: filename,
	}
}

type ResultWriter struct {
	fileName string
}

func (w *ResultWriter) Write(p []byte) (n int, err error) {
	err = ioutil.WriteFile(w.fileName, p, os.ModeAppend)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

// PatchFiles patches files and adds prefix to import paths if they have oldPrefix value at the beginning
func PatchFiles(oldPrefix, newPrefix, baseDir string, ext string) {
	err := filepath.Walk("./"+baseDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if filepath.Ext(info.Name()) == ext {
					err := patchImports(oldPrefix, newPrefix, path, baseDir)
					if err != nil {
						log.Println(err)
					}
					fmt.Println(path, info.Size())
				}
			}
			return nil
		})
	if err != nil {
		log.Println(fmt.Sprintf("error while patching: %v", err))
	}
}

func patchImports(oldPrefix, newPrefix, path string, baseDir string) error {
	appDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fileFullPath := filepath.Join(appDir, path)

	fileSet := token.NewFileSet()
	data, err := ioutil.ReadFile(fileFullPath)
	if err != nil {
		return err
	}
	f, err := parser.ParseFile(fileSet, "", string(data), parser.AllErrors)
	if err != nil {
		return err
	}
	wasPatched := false
	for i := range f.Imports {
		if strings.HasPrefix(f.Imports[i].Path.Value, "\""+oldPrefix) {
			patchImport(newPrefix, f.Imports[i])
			wasPatched = true
		}
	}

	if wasPatched {
		err = save(fileFullPath, fileSet, f)
		if err != nil {
			return fmt.Errorf("can't save file: %v", err)
		}
	}

	return nil
}

func patchImport(newPrefix string, im *ast.ImportSpec) {
	old := strings.Trim(im.Path.Value, "\"")
	newer := filepath.ToSlash(filepath.Join(newPrefix, old))
	im.Path.Value = `"` + newer + `"`
}

func save(fullPath string, fileSet *token.FileSet, f *ast.File) error {
	err := os.Rename(fullPath, fullPath+"_old")
	if err != nil {
		return fmt.Errorf("can't rename old file before saving patched version %v, error: %v", fullPath, err)
	}

	fileName := strings.TrimSuffix(fullPath, ".go") + "_patched.go"

	var buf bytes.Buffer
	err = printer.Fprint(&buf, fileSet, f)
	if err != nil {
		return fmt.Errorf("can't print file into byte buffer: %v", err)
	}
	_, err = NewResultWriter(fileName).Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("can't save result to file : %v", err)
	}
	return nil
}

// CreateProtoFiles  scans folders and subfolders starting from baseDir and generates Golang files form protobuf
func CreateProtoFiles(baseDir string, ext string) {
	err := filepath.Walk("./"+baseDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if filepath.Ext(info.Name()) == ext {
					generateProto(path, baseDir)
					fmt.Println(path, info.Size())
				}
			}
			return nil
		})
	if err != nil {
		log.Println(fmt.Sprintf("rror during creation of proto files %v", err))
	}
}

func generateProto(path string, baseDir string) {
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fullPath := filepath.Join(workDir, path)
	workDir=filepath.Join(workDir, baseDir)

	out, err := exec.Command("protoc", "-I="+workDir, fullPath, "--go_out=plugins=grpc:"+workDir).Output()
	fmt.Println(string(out))
	if err != nil {
		log.Println(err)
	}
}
