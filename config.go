package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	//"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/rclancey/argparse"
	"github.com/rclancey/logging"
	"github.com/rclancey/logrotate"
)

type SSLConfig struct {
	Port     int    `json:"port"     arg:"port"`
	CertFile string `json:"cert"     arg:"cert"`
	KeyFile  string `json:"key"      arg:"key"`
	Disabled bool   `json:"disabled" arg:"disable"`
}

func (cfg *SSLConfig) CheckCert(serverRoot string) error {
	fn, err := makeRootAbs(serverRoot, cfg.CertFile)
	if err != nil {
		return errors.Wrap(err, "can't make abs path for cert file " + cfg.CertFile)
	}
	err = checkReadableFile(fn)
	if err != nil {
		return errors.Wrapf(err, "cert file %s is not readable", fn)
	}
	cfg.CertFile = fn
	return nil
}

func (cfg *SSLConfig) CheckKey(serverRoot string) error {
	fn, err := makeRootAbs(serverRoot, cfg.KeyFile)
	if err != nil {
		return errors.Wrap(err, "can't make abs path for cert key " + cfg.KeyFile)
	}
	err = checkReadableFile(fn)
	if err != nil {
		return errors.Wrapf(err, "cert key %s is not readable", fn)
	}
	cfg.KeyFile = fn
	return nil
}

func (cfg SSLConfig) Enabled() bool {
	return !cfg.Disabled && cfg.Port != 0 && cfg.CertFile != "" && cfg.KeyFile != ""
}

func (cfg *SSLConfig) Init(serverRoot string) error {
	err := cfg.CheckCert(serverRoot)
	if err != nil {
		return errors.Wrap(err, "bad ssl cert")
	}
	err = cfg.CheckKey(serverRoot)
	if err != nil {
		return errors.Wrap(err, "bad ssl cert key")
	}
	return nil
}

type BindConfig struct {
	ExternalHostname string    `json:"hostname" arg:"hostname"`
	Port             int       `json:"port"     arg:"port"`
	SSL              SSLConfig `json:"ssl"      arg:"ssl"`
}

func (cfg *BindConfig) Init(serverRoot string) error {
	err := cfg.SSL.Init(serverRoot)
	if err != nil {
		return errors.Wrap(err, "can't configure SSL")
	}
	return nil
}

func (cfg BindConfig) Proto(ssl bool) string {
	if ssl && cfg.SSL.Enabled() {
		return "https"
	}
	return "http"
}

func (cfg BindConfig) Host(netcfg NetCfg, ssl bool) string {
	var host string
	if netcfg != nil {
		host = netcfg.GetIP().String()
	} else if cfg.ExternalHostname != "" {
		host = cfg.ExternalHostname
	} else {
		host = "localhost"
	}
	if ssl && cfg.SSL.Enabled() {
		if cfg.SSL.Port == 443 {
			return host
		}
		return host + fmt.Sprintf(":%d", cfg.SSL.Port)
	}
	if cfg.Port == 80 {
		return host
	}
	return host + fmt.Sprintf(":%d", cfg.Port)
}

func (cfg BindConfig) RootURL(netcfg NetCfg, ssl bool) *url.URL {
	return &url.URL{
		Scheme: cfg.Proto(ssl),
		Host: cfg.Host(netcfg, ssl),
		Path: "/",
	}
}

type LogConfig struct {
	Directory    string           `json:"directory"     arg:"dir"`
	AccessLog    string           `json:"access"        arg:"access"`
	ErrorLog     string           `json:"error"         arg:"error"`
	RotatePeriod int              `json:"rotate_period" arg:"rotate-period"`
	MaxSize      int64            `json:"max_size"      arg:"max-size"`
	RetainCount  int              `json:"retain"        arg:"retain"`
	LogLevel     logging.LogLevel `json:"level"         arg:"level"`
	errlog       *logging.Logger
	acclog       *logrotate.RotateFile
}

func (cfg *LogConfig) Init(serverRoot string) error {
	dn, err := makeRootAbs(serverRoot, cfg.Directory)
	if err != nil {
		return errors.Wrap(err, "can't make abs log directory " + cfg.Directory)
	}
	err = checkWritableDir(dn)
	if err != nil {
		return errors.Wrap(err, "bad log directory")
	}
	cfg.Directory = dn
	fn, err := makeRootAbs(cfg.Directory, cfg.AccessLog)
	if err != nil {
		return errors.Wrap(err, "can't make abs access log file " + cfg.AccessLog)
	}
	cfg.AccessLog = fn
	fn, err = makeRootAbs(cfg.Directory, cfg.ErrorLog)
	if err != nil {
		return errors.Wrap(err, "can't make abs error log file " + cfg.ErrorLog)
	}
	cfg.ErrorLog = fn
	return nil
}

func (cfg *LogConfig) ErrorLogger() (*logging.Logger, error) {
	if cfg.errlog == nil {
		rotlog, err := logrotate.Open(cfg.ErrorLog, time.Duration(cfg.RotatePeriod) * time.Minute, cfg.MaxSize, cfg.RetainCount)
		if err != nil {
			return nil, errors.Wrap(err, "can't create error logger")
		}
		cfg.errlog = logging.NewLogger(rotlog, cfg.LogLevel)
		logging.SetOutput(rotlog)
		logging.SetLevel(cfg.LogLevel)
	}
	return cfg.errlog, nil
}

func (cfg *LogConfig) AccessLogger() (*logrotate.RotateFile, error) {
	if cfg.acclog == nil {
		rotlog, err := logrotate.Open(cfg.AccessLog, time.Duration(cfg.RotatePeriod) * time.Minute, cfg.MaxSize, cfg.RetainCount)
		if err != nil {
			return nil, errors.Wrap(err, "can't create access logger")
		}
		cfg.acclog = rotlog
	}
	return cfg.acclog, nil
}

type ServerConfig struct {
	ConfigFile          string         `json:"-"               arg:"--config"`//,-c"`
	ServerRoot          string         `json:"server_root"     arg:"--server-root"`
	DocumentRoot        string         `json:"document_root"   arg:"--docroot"`
	DefaultProxy        string         `json:"default_proxy"   arg:"--proxy"`
	CacheDirectory      string         `json:"cache_directory" arg:"--cache-dir"`
	PidFile             string         `json:"pidfile"         arg:"--pidfile"`
	Bind                BindConfig     `json:"bind"            arg:"--bind"`
	Logging             LogConfig      `json:"log"             arg:"--log"`
	Auth                AuthConfig     `json:"auth"            arg:"--auth"`
}

func (cfg *ServerConfig) Abs(fn string) (string, error) {
	return makeRootAbs(cfg.ServerRoot, fn)
}

func (cfg *ServerConfig) ReadableFile(fn string) error {
	return checkReadableFile(fn)
}

func (cfg *ServerConfig) WritableDir(dn string) error {
	return checkWritableDir(dn)
}

func (cfg *ServerConfig) Init() error {
	p := filepath.Clean(EnvEval(cfg.ServerRoot))
	dn, err := filepath.Abs(p)
	if err != nil {
		return errors.Wrap(err, "can't make abs path for server root " + p)
	}
	cfg.ServerRoot = dn
	dn, err = cfg.Abs(cfg.DocumentRoot)
	if err != nil {
		return errors.Wrap(err, "can't make abs path for document root " + cfg.DocumentRoot)
	}
	cfg.DocumentRoot = dn
	dn, err = cfg.Abs(cfg.CacheDirectory)
	if err != nil {
		return errors.Wrap(err, "can't make abs path for cache directory " + cfg.CacheDirectory)
	}
	err = cfg.WritableDir(dn)
	if err != nil {
		return errors.Wrapf(err, "cache directory %s not writable", dn)
	}
	cfg.CacheDirectory = dn
	fn, err := cfg.Abs(cfg.PidFile)
	if err != nil {
		return errors.Wrap(err, "can't make abs path for pid file " + cfg.PidFile)
	}
	dn = filepath.Dir(fn)
	err = cfg.WritableDir(dn)
	if err != nil {
		return errors.Wrapf(err, "pid file directory %s not wriable", dn)
	}
	cfg.PidFile = fn
	err = cfg.Bind.Init(cfg.ServerRoot)
	if err != nil {
		return errors.Wrap(err, "can't configure server address")
	}
	err = cfg.Logging.Init(cfg.ServerRoot)
	if err != nil {
		return errors.Wrap(err, "can't configure logging")
	}
	err = cfg.Auth.Init(cfg.ServerRoot)
	if err != nil {
		return errors.Wrap(err, "can't configure auth")
	}
	return nil
}

func (cfg *ServerConfig) CheckPidfile() error {
	f, err := os.Open(cfg.PidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "can't stat pid file " + cfg.PidFile)
	}
	defer f.Close()
	data := make([]byte, 256)
	n, err := f.Read(data)
	if err != nil {
		return errors.Wrap(err, "can't read pid file " + cfg.PidFile)
	}
	if n == 0 {
		return nil
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(string(data[:n])), 10, 32)
	if err != nil {
		return errors.Wrap(err, "can't parse pid " + string(data[:n]))
	}
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return errors.Wrapf(err, "can't find process %d", pid)
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return errors.Errorf("%s already running at PID %d", os.Args[0], pid)
	}
	return nil
}

func (cfg *ServerConfig) CheckPorts() error {
	if cfg.Bind.Port != 0 {
		err := cfg.checkPort(cfg.Bind.Port)
		if err != nil {
			return errors.Wrap(err, "http")
		}
	}
	if cfg.Bind.SSL.Enabled() {
		err := cfg.checkPort(cfg.Bind.SSL.Port)
		if err != nil {
			return errors.Wrap(err, "https")
		}
	}
	return nil
}

func (cfg *ServerConfig) checkPort(port int) error {
	ln, err := net.Listen("tcp", ":" + strconv.Itoa(port))
	if err != nil {
		return errors.Errorf("port %d already in use", port)
	}
	ln.Close()
	return nil
}

func (cfg *ServerConfig) LoadFromFile(fn string) error {
	var f io.ReadCloser
	var err error
	if fn == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(fn)
		if err != nil {
			return errors.Wrap(err, "can't open config file " + fn)
		}
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Wrap(err, "can't read config file " + fn)
	}
	return errors.Wrap(json.Unmarshal(data, cfg), "can't decode config file " + fn)
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ConfigFile: "config.json",
		ServerRoot: ".",
		DocumentRoot: "htdocs",
		Logging: LogConfig{
			Directory: "var/log",
			AccessLog: "access.log",
			ErrorLog: "error.log",
			RotatePeriod: 24 * 60,
			RetainCount: 7,
			LogLevel: logging.INFO,
		},
		Auth: AuthConfig{
			PasswordFile: ".htpasswd",
			AuthKey: "",
			TTL: 30 * 24 * 60 * 60,
			SocialLogin: map[string]*SocialLoginConfig{},
		},
		CacheDirectory: "var/cache",
		PidFile: "var/synos.pid",
		Bind: BindConfig{
			Port: 8080,
			SSL: SSLConfig{
				Port: 8043,
			},
		},
	}
}

func Configure() (*ServerConfig, error) {
	var err error
	cfg := DefaultServerConfig()
	err = argparse.ParseArgs(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "can't parse command line arguments")
	}
	fn, err := filepath.Abs(filepath.Clean(EnvEval(cfg.ServerRoot)))
	if err != nil {
		return nil, errors.Wrap(err, "can't make abs path for server root " + cfg.ServerRoot)
	}
	cfg.ServerRoot = fn
	fn, err = makeRootAbs(cfg.ServerRoot, cfg.ConfigFile)
	if err != nil {
		return nil, errors.Wrap(err, "can't make abs path for config file " + cfg.ConfigFile)
	}
	cfg.ConfigFile = fn
	_, err = os.Stat(cfg.ConfigFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "can't stat config file " + cfg.ConfigFile)
		}
	} else {
		err = cfg.LoadFromFile(cfg.ConfigFile)
		if err != nil {
			return nil, errors.Wrap(err, "can't load config file " + cfg.ConfigFile)
		}
		err = argparse.ParseArgs(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "can't re-parse command line arguments")
		}
	}
	err = cfg.Init()
	if err != nil {
		return nil, errors.Wrap(err, "can't configure server")
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	log.Println(string(data))
	return cfg, nil
}
