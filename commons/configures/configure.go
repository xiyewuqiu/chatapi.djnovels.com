package configures

import (
	"flag"
	"im-server/commons/tools"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	EnvDev  = "dev"
	EnvProd = "prod"

	MsgStoreEngine_MySQL = "mysql"
	MsgStoreEngine_Mongo = "mongo"

	CmdMsgExpired int64 = 7 * 24 * 60 * 60 * 1000
	MsgExpired    int64 = 24 * 60 * 60 * 1000
)

type ImConfig struct {
	NodeName       string `yaml:"nodeName"`
	NodeHost       string `yaml:"nodeHost"`
	MsgStoreEngine string `yaml:"msgStoreEngine"`

	Log struct {
		LogPath        string `yaml:"logPath"`
		LogName        string `yaml:"logName"`
		LogExpireHours int    `yaml:"logExpireHours"`
	} `ymal:"log"`

	Kvdb struct {
		IsOpen   bool   `yaml:"isOpen"`
		DataPath string `yaml:"dataPath"`
	} `ymal:"kvdb"`

	MsgLogs struct {
		LogPath    string `yaml:"logPath"`
		MaxBackups int    `yaml:"maxBackups"`
		IsCompress bool   `yaml:"isCompress"`
	}

	Mysql struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Address  string `yaml:"address"`
		DbName   string `yaml:"name"`
		Debug    bool   `yaml:"debug"`
	} `yaml:"mysql"`

	MongoDb struct {
		Address string `yaml:"address"`
		DbName  string `yaml:"name"`
	} `yaml:"mongodb"`

	ConnectManager struct {
		WsPort int `yaml:"wsPort"`
	} `yaml:"connectManager"`

	ApiGateway struct {
		HttpPort int `yaml:"httpPort"`
	} `yaml:"apiGateway"`

	NavGateway struct {
		HttpPort int `yaml:"httpPort"`
	} `yaml:"navGateway"`

	AdminGateway struct {
		HttpPort int `yaml:"httpPort"`
		// BindAddr 是管理台监听地址。留空即只监听回环（安全默认值）：
		// 管理台能建租户、改策略、撤消息、翻审计，不该出现在任何网卡上。
		// 运维需要远程使用时走 SSH 隧道，不要改成 0.0.0.0。
		BindAddr string `yaml:"bindAddr"`
	} `yaml:"adminGateway"`
	AdminSecret string `yaml:"adminSecret"`

	PerformanceMetrics struct {
		IsOpen bool `yaml:"isOpen"`
	} `yaml:"performanceMetrics"`
}

func (conf ImConfig) GetApiPort() int {
	return conf.ApiGateway.HttpPort
}

// GetAdminBindAddr 返回管理台的监听地址。
//
// 刻意不返回空字符串：gin 的 Run(":8090") 等价于监听 0.0.0.0，
// 管理台能力等同根权限，任何"没配就全网卡"的默认值都是事故。
func (conf ImConfig) GetAdminBindAddr() string {
	if addr := strings.TrimSpace(conf.AdminGateway.BindAddr); addr != "" {
		return addr
	}
	return "127.0.0.1"
}

var Config ImConfig

func InitConfigures() error {
	configFile := flag.String("config", "conf/config.yml", "Path to the configuration file")
	flag.Parse()
	cfBytes, err := os.ReadFile(*configFile)
	if err == nil {
		var conf ImConfig
		yaml.Unmarshal(cfBytes, &conf)
		Config = conf
		//check
		if Config.NodeName == "" {
			Config.NodeName = tools.GenerateUUIDShort11()
		}
		if Config.NodeHost == "" {
			Config.NodeHost = "127.0.0.1"
		}
		if Config.MsgStoreEngine == "" {
			Config.MsgStoreEngine = MsgStoreEngine_MySQL
		}
		if Config.ApiGateway.HttpPort <= 0 {
			Config.ApiGateway.HttpPort = 9001
		}
		if Config.ConnectManager.WsPort <= 0 {
			Config.ConnectManager.WsPort = 9003
		}
		if Config.AdminGateway.HttpPort <= 0 {
			Config.AdminGateway.HttpPort = 8090
		}
		return nil
	} else {
		return err
	}
}
