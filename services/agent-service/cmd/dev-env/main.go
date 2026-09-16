// Command dev-env runs local development commands with a parsed dotenv file.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return errors.New("cần truyền đường dẫn dotenv và lệnh cần chạy")
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return errors.New("không xác định được đường dẫn dotenv")
	}
	if err := loadEnv(path); err != nil {
		return err
	}
	if err := os.Chdir(filepath.Dir(path)); err != nil {
		return errors.New("không truy cập được thư mục dự án")
	}
	command, err := exec.LookPath(args[1])
	if err != nil {
		return errors.New("không tìm thấy lệnh; kiểm tra công cụ đã được cài")
	}
	// Replace this process so exit codes and signals reach the invoked command.
	// #nosec G204 G702 -- This local CLI intentionally executes the developer-supplied command; it accepts no network input.
	return syscall.Exec(command, args[1:], os.Environ())
}

func loadEnv(path string) error {
	values, err := godotenv.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		// Parser errors may include the source line and therefore a secret.
		return errors.New("không đọc được dotenv; kiểm tra cú pháp và quyền đọc file")
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return errors.New("không thiết lập được biến môi trường từ dotenv")
			}
		}
	}
	return nil
}
