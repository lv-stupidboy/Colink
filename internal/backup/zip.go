// Package backup 提供数据库备份恢复功能
package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// zipDir 压缩目录为 zip 文件
func zipDir(srcDir, dstFile string) error {
	// 创建 zip 文件
	f, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// 创建 zip writer
	w := zip.NewWriter(f)
	defer w.Close()

	// 遍历目录
	baseDir := filepath.Base(srcDir)
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建 zip entry
		relPath := strings.TrimPrefix(path, srcDir)
		if relPath == "" {
			return nil
		}
		zipPath := filepath.Join(baseDir, relPath)

		if info.IsDir() {
			_, err = w.Create(zipPath + "/")
			return err
		}

		// 添加文件
		fw, err := w.Create(zipPath)
		if err != nil {
			return err
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		_, err = io.Copy(fw, fr)
		return err
	})

	return err
}

// unzipTo 解压 zip 文件到目录
func unzipTo(srcFile, dstDir string) error {
	// 打开 zip 文件
	r, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer r.Close()

	// 创建目标目录
	os.MkdirAll(dstDir, 0755)

	// 解压文件
	for _, f := range r.File {
		// 构建目标路径
		fPath := filepath.Join(dstDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fPath, 0755)
			continue
		}

		// 创建文件
		fr, err := f.Open()
		if err != nil {
			return err
		}
		defer fr.Close()

		os.MkdirAll(filepath.Dir(fPath), 0755)
		fw, err := os.Create(fPath)
		if err != nil {
			return err
		}
		defer fw.Close()

		_, err = io.Copy(fw, fr)
		if err != nil {
			return err
		}
	}

	return nil
}