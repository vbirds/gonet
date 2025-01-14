package etv3

import (
	"encoding/json"
	"fmt"
	"gonet/actor"
	"gonet/common"
	"gonet/rpc"
	"log"
	"sync"

	"go.etcd.io/etcd/clientv3"

	"golang.org/x/net/context"
)

const (
	MAILBOX_DIR     = "mailbox/"
	MAILBOX_TL_TIME = 20 * 60
)

//publish
type (
	MailBox struct {
		*common.ClusterInfo
		m_Client        *clientv3.Client
		m_Lease         clientv3.Lease
		m_MailBoxLocker *sync.RWMutex
		m_MailBoxMap    map[int64]*rpc.MailBox
	}
)

//初始化pub
func (this *MailBox) Init(endpoints []string, info *common.ClusterInfo) {
	cfg := clientv3.Config{
		Endpoints: endpoints,
	}

	etcdClient, err := clientv3.New(cfg)
	if err != nil {
		log.Fatal("Error: cannot connec to etcd:", err)
	}
	this.ClusterInfo = info
	lease := clientv3.NewLease(etcdClient)
	this.m_Client = etcdClient
	this.m_Lease = lease
	this.m_MailBoxLocker = &sync.RWMutex{}
	this.m_MailBoxMap = map[int64]*rpc.MailBox{}
	this.Start()
	this.getAll()
}

func (this *MailBox) Start() {
	go this.Run()
}

func (this *MailBox) Create(info *rpc.MailBox) bool {
	leaseResp, err := this.m_Lease.Grant(context.Background(), MAILBOX_TL_TIME)
	if err == nil {
		leaseId := leaseResp.ID
		info.LeaseId = int64(leaseId)
		key := MAILBOX_DIR + fmt.Sprintf("%d", info.Id)
		data, _ := json.Marshal(info)
		//设置key
		tx := this.m_Client.Txn(context.Background())
		tx.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
			Then(clientv3.OpPut(key, string(data), clientv3.WithLease(leaseId))).
			Else()
		txnRes, err := tx.Commit()
		return err == nil && txnRes.Succeeded
	}
	return false
}

func (this *MailBox) Lease(leaseId int64) error {
	_, err := this.m_Lease.KeepAliveOnce(context.Background(), clientv3.LeaseID(leaseId))
	return err
}

func (this *MailBox) Delete(Id int64) error {
	key := MAILBOX_DIR + fmt.Sprintf("%d", Id)
	_, err := this.m_Client.Delete(context.Background(), key)
	return err
}

func (this *MailBox) DeleteAll() error {
	_, err := this.m_Client.Delete(context.Background(), MAILBOX_DIR, clientv3.WithPrefix())
	return err
}

func (this *MailBox) add(info *rpc.MailBox) {
	this.m_MailBoxLocker.Lock()
	pMailBox, bOk := this.m_MailBoxMap[info.Id]
	if !bOk {
		this.m_MailBoxMap[info.Id] = info
	} else {
		*pMailBox = *info
	}
	this.m_MailBoxLocker.Unlock()
}

func (this *MailBox) del(info *rpc.MailBox) {
	this.m_MailBoxLocker.Lock()
	delete(this.m_MailBoxMap, int64(info.Id))
	this.m_MailBoxLocker.Unlock()
	actor.MGR.SendMsg(rpc.RpcHead{Id: info.Id}, fmt.Sprintf("%s.On_UnRegister", info.MailType.String()))
}

func (this *MailBox) Get(Id int64) *rpc.MailBox {
	this.m_MailBoxLocker.RLock()
	pMailBox, bEx := this.m_MailBoxMap[Id]
	this.m_MailBoxLocker.RUnlock()
	if bEx {
		return pMailBox
	}
	return nil
}

// subscribe
func (this *MailBox) Run() {
	wch := this.m_Client.Watch(context.Background(), MAILBOX_DIR, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for v := range wch {
		for _, v1 := range v.Events {
			if v1.Type.String() == "PUT" {
				info := nodeToMailBox(v1.Kv.Value)
				this.add(info)
			} else {
				info := nodeToMailBox(v1.PrevKv.Value)
				this.del(info)
			}
		}
	}
}

func (this *MailBox) getAll() {
	resp, err := this.m_Client.Get(context.Background(), MAILBOX_DIR, clientv3.WithPrefix())
	if err == nil && (resp != nil && resp.Kvs != nil) {
		for _, v := range resp.Kvs {
			info := nodeToMailBox(v.Value)
			this.add(info)
		}
	}
}

func nodeToMailBox(val []byte) *rpc.MailBox {
	info := &rpc.MailBox{}
	err := json.Unmarshal([]byte(val), info)
	if err != nil {
		log.Print(err)
	}
	return info
}
