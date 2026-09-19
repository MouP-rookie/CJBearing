// 文件说明：本地开发编排入口，统一执行迁移、初始化并启动 API、PC、PDA 进程。

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

func main() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		exitf("无法定位项目目录")
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	projectRoot := filepath.Dir(serverRoot)

	if _, err := os.Stat(filepath.Join(serverRoot, ".env")); err != nil {
		exitf("找不到 %s，请先复制 server\\.env.example 为 server\\.env 并填写数据库配置", filepath.Join(serverRoot, ".env"))
	}

	fmt.Println("执行数据库迁移...")
	if err := runSync(serverRoot, "go", "run", "./cmd/migrate", "up"); err != nil {
		exitf("数据库迁移失败: %v", err)
	}

	fmt.Println("初始化基础数据...")
	if err := runSync(serverRoot, "go", "run", "./cmd/seed"); err != nil {
		exitf("初始化失败: %v", err)
	}

	fmt.Println("启动 Go API...")
	api, err := startCommand(serverRoot, "go", "run", "./cmd/api")
	if err != nil {
		exitf("启动 API 失败: %v", err)
	}

	time.Sleep(2 * time.Second)

	if _, err := os.Stat(filepath.Join(projectRoot, "node_modules")); err != nil {
		fmt.Println("安装前端依赖...")
		if err := runNPM(projectRoot, "install"); err != nil {
			stopTree(api)
			exitf("安装前端依赖失败: %v", err)
		}
	}

	fmt.Println("启动 PC 前端...")
	pc, err := startNPM(projectRoot, "dev:pc")
	if err != nil {
		stopTree(api)
		exitf("启动 PC 前端失败: %v", err)
	}

	fmt.Println("启动 PDA 前端...")
	pda, err := startNPM(projectRoot, "dev:pda")
	if err != nil {
		stopTree(api)
		stopTree(pc)
		exitf("启动 PDA 前端失败: %v", err)
	}

	fmt.Println()
	fmt.Println("全部服务已启动：")
	fmt.Println("PC 端：http://127.0.0.1:5173/pc/")
	fmt.Println("PDA 端：http://127.0.0.1:5174/pda/")
	fmt.Println("API：http://127.0.0.1:8080")
	fmt.Println("管理员：admin / 1234")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止全部服务。")

	waitForStop(api, pc, pda)
}

func runSync(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	return cmd.Run()
}

func startCommand(dir, name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func runNPM(dir string, args ...string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", append([]string{"/c", "npm.cmd"}, args...)...)
	} else {
		cmd = exec.Command("npm", args...)
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startNPM(dir, script string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		return startCommand(dir, "cmd.exe", "/c", "npm.cmd", "run", script)
	}
	return startCommand(dir, "npm", "run", script)
}

func waitForStop(processes ...*exec.Cmd) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	exitCh := make(chan error, len(processes))

	for _, cmd := range processes {
		process := cmd
		go func() { exitCh <- process.Wait() }()
	}

	select {
	case sig := <-signalCh:
		fmt.Printf("\n收到停止信号 %v，正在关闭...\n", sig)
	case err := <-exitCh:
		fmt.Printf("\n某个服务已退出: %v，正在关闭其余服务...\n", err)
	}

	for _, cmd := range processes {
		stopTree(cmd)
	}
}

func stopTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
		return
	}
	_ = cmd.Process.Kill()
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
