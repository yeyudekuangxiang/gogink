package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var m = flag.String("m", "", "-m=demo.com/user")
var v = flag.String("v", "latest", "-v=latest")

const baseUrl = "https://github.com/yeyudekuangxiang/gink/archive/refs/tags/%s.zip"
const basePath = "github.com/yeyudekuangxiang/gink"

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取执行目录失败 %v", err)
		return
	}

	if *m == "" && strings.Contains(pwd, filepath.Join(os.Getenv("GOPATH"), "src")) {
		*m = pwd[len(filepath.Join(os.Getenv("GOPATH"), "src"))+1:]
		*m = strings.ReplaceAll(*m, "\\", "/")
	}

	flag.Usage = func() {
		fmt.Println(fmt.Sprintf("Usage of %s [options...] yourAppName", os.Args[0]))
		flag.PrintDefaults()
	}
	if len(os.Args) == 1 {
		flag.Usage()
		return
	}
	flag.Parse()

	//var err error
	path := os.Args[len(os.Args)-1]
	projectName := filepath.Base(path)
	projectPath := filepath.Dir(path)
	project := filepath.Join(projectPath, projectName)
	if isExist(project) {
		var d string
		fmt.Printf("项目 '%s' 已存在,是否覆盖? y/n (default:n)\n", projectName)
		_, _ = fmt.Scanln(&d)
		if d != "y" {
			return
		}
		fmt.Printf("即将删除项目 '%s' \n", projectName)
		err = os.RemoveAll(project)
		if err != nil {
			fmt.Printf("删除项目 '%s' 失败原因:%v\n", projectName, err)
			return
		}
		fmt.Printf("删除项目 '%s' 成功\n", projectName)
	}
	fmt.Printf("开始创建项目 '%s'\n", projectName)
	err = create(*v, projectPath, projectName, *m)
	if err != nil {
		fmt.Printf("创建项目 '%s' 失败原因:%v\n", projectName, err)
		return
	}

	if os.Chdir(projectName) == nil {
		/*_ = exec.Command("go", "mod", "download").Run()*/
		_ = exec.Command("gofmt", "-w", "-s", "-l").Run()
	}
	fmt.Printf("创建项目 '%s' 成功\n", projectName)
}
func isExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}
func down(v string, path string, projectName string) (projectPath string, err error) {
	fmt.Println("下载中...")
	if v != "latest" {
		v = "v" + v
	}
	resp, err := http.Get(fmt.Sprintf(baseUrl, v))
	if err != nil {
		return "", errors.New("获取失败")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("版本不存在")
	}
	defer resp.Body.Close()

	if !isExist(path) {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			return "", errors.New("创建目录失败")
		}
	}
	zipPath := filepath.Join(path, v+".zip")
	if isExist(zipPath) {
		_ = os.RemoveAll(zipPath)
	}
	file, err := os.Create(zipPath)
	if err != nil {
		return "", errors.New("创建压缩包失败")
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", errors.New("保存压缩包失败")
	}
	fmt.Println("下载完成")
	fmt.Println("解压中...")
	err = Unzip(zipPath, path)
	if err != nil {
		return "", errors.New("解压失败")
	}
	err = os.Rename(filepath.Join(path, "gink-"+v), filepath.Join(path, projectName))
	if err != nil {
		return "", errors.New("重命名项目失败")
	}
	_ = file.Close()
	err = os.Remove(zipPath)
	fmt.Println("解压完成")
	return filepath.Join(path, projectName), nil
}
func create(v string, path, projectName, modPath string) error {
	projectPath, err := down(v, path, projectName)
	if err != nil {
		return err
	}
	mod := projectName
	if modPath != "" {
		mod = modPath + "/" + mod
	}
	fmt.Println("替换包名中...")
	err = replace(projectPath, func(file *os.File) error {
		defer file.Close()
		bs, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		fileContent := strings.ReplaceAll(string(bs), basePath, mod)
		fileContent = strings.ReplaceAll(fileContent, "gink", projectName)
		_ = file.Truncate(0)
		_, _ = file.Seek(0, 0)
		_, err = file.WriteString(fileContent)
		return err
	})
	if err != nil {
		return err
	}
	fmt.Println("替换完成")
	return nil
}
func replace(path string, call func(file *os.File) error) error {
	list, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	for _, item := range list {
		if item.IsDir() {
			err = replace(filepath.Join(path, item.Name()), call)
			if err != nil {
				return err
			}
		} else {
			f, err := os.OpenFile(filepath.Join(path, item.Name()), os.O_RDWR, 0766)
			if err != nil {
				return err
			}
			err = call(f)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func Unzip(zipFile string, destDir string) error {
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, os.ModePerm)
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return err
			}

			inFile, err := f.Open()
			if err != nil {
				return err
			}
			defer inFile.Close()

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, inFile)
			if err != nil {
				return err
			}
			_ = outFile.Close()
			_ = inFile.Close()
		}
	}
	return nil
}
