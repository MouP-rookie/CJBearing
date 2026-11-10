// 文件说明：本地开发编排入口，统一启动 API、QMS、门户、PC 和 PDA。

package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var developmentPorts = []int{8081, 5173, 5174, 5175}

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
	apiPort := configuredPort(filepath.Join(serverRoot, ".env"), "HTTP_ADDR", 8090)
	ports := append([]int{apiPort}, developmentPorts...)
	if occupied := occupiedPorts(ports); len(occupied) > 0 {
		exitf("以下端口已被占用，请先运行 scripts\\stop-all.ps1 或关闭对应服务：%s", joinPorts(occupied))
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

	fmt.Println("启动 QMS Go 服务...")
	qms, err := startCommand(serverRoot, "go", "run", "./cmd/qms")
	if err != nil {
		stopTree(api)
		exitf("启动 QMS 服务失败: %v", err)
	}

	time.Sleep(2 * time.Second)

	if _, err := os.Stat(filepath.Join(projectRoot, "node_modules")); err != nil {
		fmt.Println("安装前端依赖...")
		if err := runNPM(projectRoot, "install"); err != nil {
			stopTree(api)
			stopTree(qms)
			exitf("安装前端依赖失败: %v", err)
		}
	}

	fmt.Println("启动统一门户...")
	portal, err := startFrontend(projectRoot, "web-portal", "dev:portal")
	if err != nil {
		stopTree(api)
		stopTree(qms)
		exitf("启动统一门户失败: %v", err)
	}

	fmt.Println("启动 PC 前端...")
	pc, err := startFrontend(projectRoot, "web-pc", "dev:pc")
	if err != nil {
		stopTree(api)
		stopTree(qms)
		stopTree(portal)
		exitf("启动 PC 前端失败: %v", err)
	}

	fmt.Println("启动 PDA 前端...")
	pda, err := startFrontend(projectRoot, "web-pda", "dev:pda")
	if err != nil {
		stopTree(api)
		stopTree(qms)
		stopTree(portal)
		stopTree(pc)
		exitf("启动 PDA 前端失败: %v", err)
	}

	fmt.Println("等待全部服务端口就绪...")
	if missing := waitForPorts(ports, 60*time.Second); len(missing) > 0 {
		stopTree(api)
		stopTree(qms)
		stopTree(portal)
		stopTree(pc)
		stopTree(pda)
		exitf("以下服务未能在 60 秒内监听端口：%s", joinPorts(missing))
	}

	fmt.Println()
	fmt.Println("全部服务已启动：")
	fmt.Println("统一门户：http://127.0.0.1:5175/portal/")
	fmt.Println("PC 端：http://127.0.0.1:5173/pc/")
	fmt.Println("PDA 端：http://127.0.0.1:5174/pda/")
	fmt.Printf("API：http://127.0.0.1:%d\n", apiPort)
	fmt.Println("QMS 系统：http://127.0.0.1:8081")
	fmt.Println("管理员：admin / 1234")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止全部服务。")

	waitForStop(api, qms, portal, pc, pda)
}

func configuredPort(path, key string, fallback int) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	for _, line := range strings.Split(string(content), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || name != key {
			continue
		}
		_, port, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimSpace(value), ":"))
		if err == nil {
			parsed, parseErr := strconv.Atoi(port)
			if parseErr == nil && parsed > 0 {
				return parsed
			}
		}
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil && parsed > 0 {
			return parsed
		}
		if strings.HasPrefix(strings.TrimSpace(value), ":") {
			if parsed, parseErr := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(value), ":")); parseErr == nil && parsed > 0 {
				return parsed
			}
		}
		return fallback
	}
	return fallback
}

func occupiedPorts(ports []int) []int {
	occupied := []int{}
	for _, port := range ports {
		listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			occupied = append(occupied, port)
			continue
		}
		_ = listener.Close()
	}
	return occupied
}

func waitForPorts(ports []int, timeout time.Duration) []int {
	pending := make(map[int]bool, len(ports))
	for _, port := range ports {
		pending[port] = true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) && len(pending) > 0 {
		for port := range pending {
			conn, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				delete(pending, port)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	missing := make([]int, 0, len(pending))
	for port := range pending {
		missing = append(missing, port)
	}
	sort.Ints(missing)
	return missing
}

func joinPorts(ports []int) string {
	values := make([]string, 0, len(ports))
	for _, port := range ports {
		values = append(values, strconv.Itoa(port))
	}
	return strings.Join(values, ", ")
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

func startFrontend(dir, workspace, script string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("npm.cmd"); err == nil {
			return startCommand(dir, "cmd.exe", "/c", "npm.cmd", "run", script)
		}
		node, err := exec.LookPath("node.exe")
		if err != nil {
			// 与 start-all.ps1 一样，支持本机已有的 DSH Node 运行时。
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return nil, fmt.Errorf("找不到 npm.cmd 或 node.exe: %w", err)
			}
			node = filepath.Join(home, ".dsh", "dsh-runtimes", "dsh-next-primary-runtime", "dependencies", "node", "bin", "node.exe")
			if _, err := os.Stat(node); err != nil {
				return nil, fmt.Errorf("找不到 npm.cmd 或 node.exe: %w", err)
			}
		}
		vite := filepath.Join(dir, "node_modules", "vite", "bin", "vite.js")
		if _, err := os.Stat(vite); err != nil {
			return nil, fmt.Errorf("找不到 Vite，请安装前端依赖: %w", err)
		}
		return startCommand(filepath.Join(dir, workspace), node, vite, "--host", "0.0.0.0")
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
