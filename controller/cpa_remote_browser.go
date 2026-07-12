package controller

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	cpaRemoteBrowserWidth  = 1440
	cpaRemoteBrowserHeight = 900
	cpaRemoteBrowserBinary = "/opt/new-api/remote-browser/chrome-linux64/chrome"
	cpaRemoteBrowserXvfb   = "/opt/new-api/remote-browser/full-deps/usr/bin/Xvfb-qapi"
	cpaRemoteBrowserXBin   = "/opt/new-api/remote-browser/full-deps/usr/bin"
	cpaRemoteBrowserRoot   = "/opt/new-api/remote-browser/sessions"
	cpaRemoteBrowserLibs   = "/opt/new-api/remote-browser/full-deps/usr/lib/x86_64-linux-gnu:/opt/new-api/remote-browser/full-deps/lib/x86_64-linux-gnu:/opt/new-api/remote-browser/deps/usr/lib/x86_64-linux-gnu:/opt/new-api/remote-browser/deps/lib/x86_64-linux-gnu"
)

type cpaRemoteBrowserSession struct {
	state       string
	userID      int
	debugPort   int
	display     int
	profileDir  string
	process     *os.Process
	xvfbProcess *os.Process
	logFile     *os.File
	connection  *websocket.Conn
	nextID      int
	callMutex   sync.Mutex
	closeOnce   sync.Once
}

type cpaRemoteBrowserInputRequest struct {
	Type    string  `json:"type"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	DeltaX  float64 `json:"delta_x,omitempty"`
	DeltaY  float64 `json:"delta_y,omitempty"`
	Text    string  `json:"text,omitempty"`
	Key     string  `json:"key,omitempty"`
	Code    string  `json:"code,omitempty"`
	KeyCode int     `json:"key_code,omitempty"`
}

type cdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

var cpaRemoteBrowserManager struct {
	sync.Mutex
	active *cpaRemoteBrowserSession
}

var cpaRemoteBrowserSpecialKeys = map[string]struct {
	code    string
	keyCode int
}{
	"ArrowDown":  {code: "ArrowDown", keyCode: 40},
	"ArrowLeft":  {code: "ArrowLeft", keyCode: 37},
	"ArrowRight": {code: "ArrowRight", keyCode: 39},
	"ArrowUp":    {code: "ArrowUp", keyCode: 38},
	"Backspace":  {code: "Backspace", keyCode: 8},
	"Delete":     {code: "Delete", keyCode: 46},
	"Enter":      {code: "Enter", keyCode: 13},
	"Escape":     {code: "Escape", keyCode: 27},
	"Tab":        {code: "Tab", keyCode: 9},
}

func startCPARemoteBrowser(state string, userID int, authorizationURL string) error {
	cpaRemoteBrowserManager.Lock()
	defer cpaRemoteBrowserManager.Unlock()

	if cpaRemoteBrowserManager.active != nil {
		return errors.New("已有远程官方登录会话，请先关闭后重试")
	}
	if !validCPAOAuthState(state) || userID <= 0 || !validCPAOAuthAuthorizationURL(authorizationURL) {
		return errors.New("远程浏览器登录参数无效")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("分配远程浏览器端口失败: %w", err)
	}
	debugPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	display, err := findCPARemoteBrowserDisplay()
	if err != nil {
		return err
	}

	profileDir := filepath.Join(cpaRemoteBrowserRoot, state)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("创建远程浏览器会话失败: %w", err)
	}
	logFile, err := os.OpenFile(filepath.Join(profileDir, "browser.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = os.RemoveAll(profileDir)
		return fmt.Errorf("创建远程浏览器日志失败: %w", err)
	}

	xvfbCommand := exec.Command(
		cpaRemoteBrowserXvfb,
		fmt.Sprintf(":%d", display),
		"-screen", "0", fmt.Sprintf("%dx%dx24", cpaRemoteBrowserWidth, cpaRemoteBrowserHeight),
		"-nolisten", "tcp",
		"-ac",
	)
	xvfbCommand.Dir = cpaRemoteBrowserXBin
	xvfbCommand.Env = append(os.Environ(), "LD_LIBRARY_PATH="+cpaRemoteBrowserLibs)
	xvfbCommand.Stdout = logFile
	xvfbCommand.Stderr = logFile
	if err := xvfbCommand.Start(); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(profileDir)
		return fmt.Errorf("启动远程浏览器显示器失败: %w", err)
	}

	session := &cpaRemoteBrowserSession{
		state:       state,
		userID:      userID,
		debugPort:   debugPort,
		display:     display,
		profileDir:  profileDir,
		xvfbProcess: xvfbCommand.Process,
		logFile:     logFile,
	}
	cpaRemoteBrowserManager.active = session
	go func() {
		_ = xvfbCommand.Wait()
		releaseCPARemoteBrowser(session)
	}()

	if err := waitCPARemoteBrowserDisplay(display); err != nil {
		cpaRemoteBrowserManager.active = nil
		session.close()
		return err
	}

	command := exec.Command(
		cpaRemoteBrowserBinary,
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-features=Translate",
		"--no-first-run",
		"--no-default-browser-check",
		fmt.Sprintf("--window-size=%d,%d", cpaRemoteBrowserWidth, cpaRemoteBrowserHeight),
		"--remote-debugging-address=127.0.0.1",
		fmt.Sprintf("--remote-debugging-port=%d", debugPort),
		"--remote-allow-origins=*",
		"--user-data-dir="+profileDir,
		authorizationURL,
	)
	command.Env = append(os.Environ(),
		"DISPLAY="+fmt.Sprintf(":%d", display),
		"LD_LIBRARY_PATH="+cpaRemoteBrowserLibs,
	)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		cpaRemoteBrowserManager.active = nil
		session.close()
		return fmt.Errorf("启动远程浏览器失败: %w", err)
	}

	session.process = command.Process
	go func() {
		_ = command.Wait()
		releaseCPARemoteBrowser(session)
	}()

	if err := session.connect(); err != nil {
		cpaRemoteBrowserManager.active = nil
		session.close()
		return err
	}
	return nil
}

func findCPARemoteBrowserDisplay() (int, error) {
	for display := 100; display < 200; display++ {
		lockPath := fmt.Sprintf("/tmp/.X%d-lock", display)
		socketPath := fmt.Sprintf("/tmp/.X11-unix/X%d", display)
		if _, err := os.Stat(lockPath); err == nil || !os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(socketPath); err == nil || !os.IsNotExist(err) {
			continue
		}
		return display, nil
	}
	return 0, errors.New("没有可用的远程浏览器显示器")
}

func waitCPARemoteBrowserDisplay(display int) error {
	socketPath := fmt.Sprintf("/tmp/.X11-unix/X%d", display)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("远程浏览器显示器启动超时")
}

func (session *cpaRemoteBrowserSession) connect() error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(12 * time.Second)
	var websocketURL string
	for time.Now().Before(deadline) {
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", session.debugPort))
		if err == nil {
			var targets []struct {
				Type                 string `json:"type"`
				URL                  string `json:"url"`
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			decodeErr := common.DecodeJson(response.Body, &targets)
			_ = response.Body.Close()
			if decodeErr == nil {
				for _, target := range targets {
					if target.Type == "page" && target.WebSocketDebuggerURL != "" {
						websocketURL = target.WebSocketDebuggerURL
						if validCPAOAuthAuthorizationURL(target.URL) {
							break
						}
					}
				}
			}
		}
		if websocketURL != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if websocketURL == "" {
		return errors.New("远程浏览器启动超时")
	}

	connection, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		return fmt.Errorf("连接远程浏览器失败: %w", err)
	}
	session.connection = connection
	if err := session.call("Page.enable", map[string]any{}, nil); err != nil {
		return err
	}
	return session.call("Emulation.setDeviceMetricsOverride", map[string]any{
		"width":             cpaRemoteBrowserWidth,
		"height":            cpaRemoteBrowserHeight,
		"deviceScaleFactor": 1,
		"mobile":            false,
	}, nil)
}

func (session *cpaRemoteBrowserSession) call(method string, params any, output any) error {
	session.callMutex.Lock()
	defer session.callMutex.Unlock()
	if session.connection == nil {
		return errors.New("远程浏览器连接不可用")
	}
	_ = session.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_ = session.connection.SetReadDeadline(time.Now().Add(10 * time.Second))
	session.nextID++
	requestID := session.nextID
	payload, err := common.Marshal(map[string]any{"id": requestID, "method": method, "params": params})
	if err != nil {
		return err
	}
	if err := session.connection.WriteMessage(websocket.TextMessage, payload); err != nil {
		return err
	}
	for {
		_, message, err := session.connection.ReadMessage()
		if err != nil {
			return err
		}
		var response cdpResponse
		if err := common.Unmarshal(message, &response); err != nil || response.ID != requestID {
			continue
		}
		if response.Error != nil {
			return errors.New(response.Error.Message)
		}
		if output != nil && len(response.Result) > 0 {
			return common.Unmarshal(response.Result, output)
		}
		return nil
	}
}

func (session *cpaRemoteBrowserSession) close() {
	session.closeOnce.Do(func() {
		if session.connection != nil {
			_ = session.connection.Close()
		}
		if session.process != nil {
			_ = session.process.Kill()
		}
		if session.xvfbProcess != nil {
			_ = session.xvfbProcess.Kill()
		}
		if session.logFile != nil {
			_ = session.logFile.Close()
		}
		if strings.HasPrefix(session.profileDir, cpaRemoteBrowserRoot+string(os.PathSeparator)) {
			_ = os.RemoveAll(session.profileDir)
		}
	})
}

func releaseCPARemoteBrowser(session *cpaRemoteBrowserSession) {
	cpaRemoteBrowserManager.Lock()
	if cpaRemoteBrowserManager.active == session {
		cpaRemoteBrowserManager.active = nil
	}
	cpaRemoteBrowserManager.Unlock()
	session.close()
}

func getCPARemoteBrowser(state string, userID int) (*cpaRemoteBrowserSession, error) {
	cpaRemoteBrowserManager.Lock()
	defer cpaRemoteBrowserManager.Unlock()
	session := cpaRemoteBrowserManager.active
	if session == nil || session.state != state || session.userID != userID {
		return nil, errors.New("远程浏览器会话不存在")
	}
	return session, nil
}

func stopCPARemoteBrowser(state string, userID int) {
	cpaRemoteBrowserManager.Lock()
	defer cpaRemoteBrowserManager.Unlock()
	session := cpaRemoteBrowserManager.active
	if session == nil || session.state != state || session.userID != userID {
		return
	}
	cpaRemoteBrowserManager.active = nil
	session.close()
}

func validateCPARemoteBrowserInput(request cpaRemoteBrowserInputRequest) error {
	finite := func(value float64) bool {
		return !math.IsNaN(value) && !math.IsInf(value, 0)
	}
	validPoint := finite(request.X) && finite(request.Y) &&
		request.X >= 0 && request.X <= cpaRemoteBrowserWidth &&
		request.Y >= 0 && request.Y <= cpaRemoteBrowserHeight

	switch request.Type {
	case "click":
		if !validPoint {
			return errors.New("点击坐标超出范围")
		}
	case "scroll":
		if !validPoint || !finite(request.DeltaX) || !finite(request.DeltaY) ||
			math.Abs(request.DeltaX) > 10000 || math.Abs(request.DeltaY) > 10000 {
			return errors.New("滚动输入无效")
		}
	case "text":
		if request.Text == "" || len(request.Text) > 4096 {
			return errors.New("输入文本无效")
		}
	case "key":
		expected, ok := cpaRemoteBrowserSpecialKeys[request.Key]
		if !ok || request.Code != expected.code || request.KeyCode != expected.keyCode {
			return errors.New("按键输入无效")
		}
	default:
		return errors.New("不支持的远程浏览器输入类型")
	}
	return nil
}

func GetCPAOAuthBrowserFrame(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	if err := validateCPAOAuthSession(c, state); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		return
	}
	session, err := getCPARemoteBrowser(state, c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		return
	}
	var result struct {
		Data string `json:"data"`
	}
	if err := session.call("Page.captureScreenshot", map[string]any{
		"format":      "jpeg",
		"quality":     80,
		"fromSurface": true,
	}, &result); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "获取远程浏览器画面失败"})
		return
	}
	image, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "远程浏览器画面格式错误"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", image)
}

func SendCPAOAuthBrowserInput(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	if err := validateCPAOAuthSession(c, state); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		return
	}
	session, err := getCPARemoteBrowser(state, c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		return
	}
	var request cpaRemoteBrowserInputRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "远程浏览器输入无效"})
		return
	}
	if err := validateCPARemoteBrowserInput(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	switch request.Type {
	case "click":
		for _, eventType := range []string{"mousePressed", "mouseReleased"} {
			if err := session.call("Input.dispatchMouseEvent", map[string]any{
				"type":       eventType,
				"x":          request.X,
				"y":          request.Y,
				"button":     "left",
				"clickCount": 1,
			}, nil); err != nil {
				common.ApiError(c, err)
				return
			}
		}
	case "scroll":
		if err := session.call("Input.dispatchMouseEvent", map[string]any{
			"type":   "mouseWheel",
			"x":      request.X,
			"y":      request.Y,
			"deltaX": request.DeltaX,
			"deltaY": request.DeltaY,
		}, nil); err != nil {
			common.ApiError(c, err)
			return
		}
	case "text":
		if err := session.call("Input.insertText", map[string]any{"text": request.Text}, nil); err != nil {
			common.ApiError(c, err)
			return
		}
	case "key":
		for _, eventType := range []string{"keyDown", "keyUp"} {
			if err := session.call("Input.dispatchKeyEvent", map[string]any{
				"type":                  eventType,
				"key":                   request.Key,
				"code":                  request.Code,
				"windowsVirtualKeyCode": request.KeyCode,
				"nativeVirtualKeyCode":  request.KeyCode,
			}, nil); err != nil {
				common.ApiError(c, err)
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
