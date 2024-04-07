package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/metacubex/mihomo/adapter/outbound"
	"math/rand"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/timerzz/proxypool/pkg/tool"
)

var (
	ErrorNotVmessLink          = errors.New("not a correct vmess link")
	ErrorVmessPayloadParseFail = errors.New("vmess link payload parse failed")
)

type Vmess struct {
	outbound.VmessOption
	Country string `proxy:"count"`
	Usable  bool   `proxy:"usable"`
	Type    string `proxy:"type,omitempty"`
}

func (v *Vmess) SetName(name string) {
	v.Name = name
}

func (v *Vmess) AddToName(name string) {
	v.Name += name
}

func (v *Vmess) SetIP(ip string) {
	v.Server = ip
}

func (v *Vmess) TypeName() string {
	return v.Type
}

func (v *Vmess) BaseInfo() *Base {
	return &Base{
		Name:    v.Name,
		Server:  v.Server,
		Type:    v.Type,
		Country: v.Country,
		Port:    v.Port,
		UDP:     v.UDP,
		Usable:  v.Usable,
	}
}

func (v *Vmess) SetUsable(usable bool) {
	v.Usable = usable
}

func (v *Vmess) SetCountry(country string) {
	v.Country = country
}

func (v *Vmess) Identifier() string {
	return net.JoinHostPort(v.Server, strconv.Itoa(v.Port)) + v.Cipher + v.UUID
}

func (v *Vmess) String() string {
	data, err := jsoniter.Config{TagKey: "proxy"}.Froze().MarshalToString(v)
	//data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return data
}

func (v *Vmess) ToClash() string {
	return "- " + v.String()
}

func (v *Vmess) Clone() Proxy {
	return v
}

func (v *Vmess) Link() (link string) {
	vjv, err := json.Marshal(v.toLinkJson())
	if err != nil {
		return
	}
	return fmt.Sprintf("vmess://%s", tool.Base64EncodeBytes(vjv))
}

type vmessLinkJson struct {
	Add      string `json:"add"`
	V        string `json:"v"`
	Ps       string `json:"ps"`
	Port     int    `json:"port"`
	Id       string `json:"id"`
	Aid      string `json:"aid"`
	Net      string `json:"net"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Path     string `json:"path"`
	Tls      string `json:"tls"`
	Security string `json:"scy"`
	SNI      string `json:"sni"`
	Alpn     string `json:"alpn"`
	Fp       string `json:"fp"`
}

func (v *Vmess) toLinkJson() vmessLinkJson {
	vj := vmessLinkJson{
		Add:  v.Server,
		Ps:   v.Name,
		Port: v.Port,
		Id:   v.UUID,
		Aid:  strconv.Itoa(v.AlterID),
		Net:  v.Network,
		Host: v.ServerName,
		V:    "2",
	}
	if v.TLS {
		vj.Tls = "tls"
	}
	if v.Network == "ws" {
		vj.Path = v.WSOpts.Path
		if host, ok := v.WSOpts.Headers["HOST"]; ok && host != "" {
			vj.Host = host
		}
	}
	return vj
}

func ParseVmessLink(link string) (*Vmess, error) {
	if !strings.HasPrefix(link, "vmess") {
		return nil, ErrorNotVmessLink
	}

	vmessmix := strings.SplitN(link, "://", 2)
	if len(vmessmix) < 2 {
		return nil, ErrorNotVmessLink
	}
	linkPayload := vmessmix[1]
	if strings.Contains(linkPayload, "?") {
		// 使用第二种解析方法 目测是Shadowrocket格式
		var infoPayloads []string
		if strings.Contains(linkPayload, "/?") {
			infoPayloads = strings.SplitN(linkPayload, "/?", 2)
		} else {
			infoPayloads = strings.SplitN(linkPayload, "?", 2)
		}
		if len(infoPayloads) < 2 {
			return nil, ErrorNotVmessLink
		}

		baseInfo, err := tool.Base64DecodeString(infoPayloads[0])
		if err != nil {
			return nil, ErrorVmessPayloadParseFail
		}
		baseInfoPath := strings.Split(baseInfo, ":")
		if len(baseInfoPath) < 3 {
			return nil, ErrorPathNotComplete
		}
		// base info
		cipher := baseInfoPath[0]
		mixInfo := strings.SplitN(baseInfoPath[1], "@", 2)
		if len(mixInfo) < 2 {
			return nil, ErrorVmessPayloadParseFail
		}
		uuid := mixInfo[0]
		server := mixInfo[1]
		portStr := baseInfoPath[2]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, ErrorVmessPayloadParseFail
		}

		moreInfo, _ := url.ParseQuery(infoPayloads[1])
		remarks := moreInfo.Get("remarks")

		// Transmission protocol
		wsHeaders := make(map[string]string)
		h2Opt := outbound.HTTP2Options{
			Host: make([]string, 0),
		}
		httpOpt := outbound.HTTPOptions{}

		// Network <- obfs=websocket
		obfs := moreInfo.Get("obfs")
		network := "tcp"
		if obfs == "http" {
			httpOpt.Method = "GET" // 不知道Headers为空时会不会报错
		}
		if obfs == "websocket" {
			network = "ws"
		} else { // when http h2
			network = obfs
		}
		// HTTP Object: Host <- obfsParam=www.036452916.xyz
		host := moreInfo.Get("obfsParam")
		if host != "" {
			switch obfs {
			case "websocket":
				wsHeaders["Host"] = host
			case "h2":
				h2Opt.Host = append(h2Opt.Host, host)
			}
		}
		// HTTP Object: Path
		path := moreInfo.Get("path")
		if path == "" {
			path = "/"
		}
		switch obfs {
		case "h2":
			h2Opt.Path = path
			path = ""
		case "http":
			httpOpt.Path = append(httpOpt.Path, path)
			path = ""
		}

		tls := moreInfo.Get("tls") == "1"
		if obfs == "h2" {
			tls = true
		}
		// allowInsecure=1 Clash config unsuported
		// alterId=64
		aid := 0
		aidStr := moreInfo.Get("alterId")
		if aidStr != "" {
			aid, _ = strconv.Atoi(aidStr)
		}

		v := Vmess{
			Type: "vmess",
			VmessOption: outbound.VmessOption{
				Name:           remarks + "_" + strconv.Itoa(rand.Int()),
				Server:         server,
				Port:           port,
				UUID:           uuid,
				AlterID:        aid,
				Cipher:         cipher,
				Network:        network,
				TLS:            tls,
				SkipCertVerify: true,
				ServerName:     server,
				HTTPOpts:       httpOpt,
				HTTP2Opts:      h2Opt,
			},
		}
		if v.Network == "ws" {
			v.WSOpts = outbound.WSOptions{
				Path:    path,
				Headers: wsHeaders,
			}
		}

		return &v, nil
	} else {
		// V2rayN ref: https://github.com/2dust/v2rayN/wiki/%E5%88%86%E4%BA%AB%E9%93%BE%E6%8E%A5%E6%A0%BC%E5%BC%8F%E8%AF%B4%E6%98%8E(ver-2)
		payload, err := tool.Base64DecodeString(linkPayload)
		if err != nil {
			return nil, ErrorVmessPayloadParseFail
		}
		vmessJson := vmessLinkJson{}
		jsonMap, err := str2jsonDynaUnmarshal(payload)
		if err != nil {
			return nil, err
		}
		vmessJson, err = mapStrInter2VmessLinkJson(jsonMap)
		if err != nil {
			return nil, err
		}

		alterId, err := strconv.Atoi(vmessJson.Aid)
		if err != nil {
			alterId = 0
		}
		tls := vmessJson.Tls == "tls"

		if vmessJson.Net == "h2" {
			tls = true
		}

		//h2Opt := outbound.HTTP2Options{}
		//httpOpt := outbound.HTTPOptions{}

		v := Vmess{
			Type: "vmess",
			VmessOption: outbound.VmessOption{
				Server:         vmessJson.Add,
				Port:           vmessJson.Port,
				UDP:            false,
				UUID:           vmessJson.Id,
				AlterID:        alterId,
				Cipher:         "auto",
				Network:        vmessJson.Net,
				ServerName:     vmessJson.SNI,
				TLS:            tls,
				SkipCertVerify: true,
				Fingerprint:    vmessJson.Fp,
				ALPN:           strings.Split(vmessJson.Alpn, ","),
			},
		}

		switch v.Network {
		case "http":
			if vmessJson.Path == "" {
				vmessJson.Path = "/"
			}
			v.HTTPOpts.Method = "GET"
			v.HTTPOpts.Path = append(v.HTTPOpts.Path, vmessJson.Path)
		case "h2":
			if vmessJson.Path == "" {
				vmessJson.Path = "/"
			}
			if vmessJson.Host != "" {
				v.HTTP2Opts.Host = append(v.HTTP2Opts.Host, vmessJson.Host)
			}
			v.HTTP2Opts.Path = vmessJson.Path

		case "ws":
			wsHeaders := make(map[string]string)
			wsHeaders["HOST"] = vmessJson.Host
			v.WSOpts.Path = vmessJson.Path
			v.WSOpts.Headers = wsHeaders
		}

		return &v, nil
	}
}

var (
	vmessPlainRe = regexp.MustCompile("vmess://([A-Za-z0-9+/_?&=-])+")
)

func GrepVmessLinkFromString(text string) []string {
	results := make([]string, 0)
	if !strings.Contains(text, "vmess://") {
		return results
	}
	texts := strings.Split(text, "vmess://")
	for _, text := range texts {
		results = append(results, vmessPlainRe.FindAllString("vmess://"+text, -1)...)
	}
	return results
}

func str2jsonDynaUnmarshal(s string) (jsn map[string]interface{}, err error) {
	var f interface{}
	err = json.Unmarshal([]byte(s), &f)
	if err != nil {
		return nil, err
	}
	jsn, ok := f.(map[string]interface{}) // f is pointer point to map struct
	if !ok {
		return nil, ErrorVmessPayloadParseFail
	}
	return jsn, err
}

func mapStrInter2VmessLinkJson(jsn map[string]interface{}) (vmessLinkJson, error) {
	vmess := vmessLinkJson{}
	var err error

	vmessVal := reflect.ValueOf(&vmess).Elem()
	for i := 0; i < vmessVal.NumField(); i++ {
		tags := vmessVal.Type().Field(i).Tag.Get("json")
		tag := strings.Split(tags, ",")
		if jsnVal, ok := jsn[strings.ToLower(tag[0])]; ok {
			if strings.ToLower(tag[0]) == "port" { // set int in port
				switch jsnVal := jsnVal.(type) {
				case float64:
					vmessVal.Field(i).SetInt(int64(jsnVal))
				case string: // Force Convert
					valInt, err := strconv.Atoi(jsnVal)
					if err != nil {
						valInt = 443
					}
					vmessVal.Field(i).SetInt(int64(valInt))
				default:
					vmessVal.Field(i).SetInt(443)
				}
			} else if strings.ToLower(tag[0]) == "ps" {
				continue
			} else { // set string in other fields
				switch jsnVal := jsnVal.(type) {
				case string:
					vmessVal.Field(i).SetString(jsnVal)
				default: // Force Convert
					vmessVal.Field(i).SetString(fmt.Sprintf("%v", jsnVal))
				}
			}
		}
	}
	return vmess, err
}
