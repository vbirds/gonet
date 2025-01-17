 package login

 import (
	 "gonet/base"
	 "gonet/base/ini"
	 "gonet/common"
	 "net/http"
 )

type(
	ServerMgr struct{
		m_Inited      bool
		m_config      ini.Config
		m_FileMonitor common.IFileMonitor
	}

	IServerMgr interface{
		Init() bool
		GetFileMonitor() common.IFileMonitor
	}

	Config struct {
		common.Http	`yaml:"login"`
	}
)

var(
	CONF Config
	SERVER ServerMgr
)

func (this *ServerMgr)Init() bool{
	if(this.m_Inited){
		return true
	}

	//初始配置文件
	base.ReadConf("gonet.yaml", &CONF)

	//动态监控文件改变
	this.m_FileMonitor = &common.FileMonitor{}
	this.m_FileMonitor.Init()

	NETGATECONF.Init()

	http.HandleFunc("/login/", GetNetGateS)
	http.ListenAndServe(CONF.Http.Listen, nil)
	return  false
}

 func (this *ServerMgr) GetFileMonitor() common.IFileMonitor {
	 return this.m_FileMonitor
 }