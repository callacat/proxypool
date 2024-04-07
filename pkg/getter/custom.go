package getter

import (
	"fmt"
	"github.com/timerzz/proxypool/log"
	"github.com/timerzz/proxypool/pkg/proxy"
	"github.com/timerzz/proxypool/pkg/tool"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
)

func init() {
	Register("custom", NewCustomPlugin)
}

// CustomPlugin 自定义插件
// Exec为插件的可执行文件路劲
// Args为执行参数
// 插件暂时没有硬性要求，执行结果的返回值是订阅链接或者分享链接即可，每行一个链接
type CustomPlugin struct {
	Exec string
	Args string
}

func (c *CustomPlugin) Get() (result proxy.ProxyList) {
	cmd := exec.Command(c.Exec, strings.Split(c.Args, " ")...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error(fmt.Sprintf("%s 执行失败:%s", c.Exec, out))
		return nil
	}
	// 插件的输出每一行为一条记录，可以是分享链接，也可以是订阅地址
	// 如果是http开头的字符串，认为是订阅地址，否则认为是分享链接
	for _, s := range strings.Fields(string(out)) {
		if strings.HasPrefix(s, "http") {
			newResult := (&Subscribe{Url: s}).Get()
			if len(newResult) == 0 {
				newResult = (&Clash{Url: s}).Get()
			}
			result = result.UniqAppendProxyList(newResult)
		} else {
			p, err := proxy.ParseProxyFromLink(s)
			if err == nil && p != nil {
				result = append(result, p)
			}
		}
	}
	return
}

func (c *CustomPlugin) Get2ChanWG(pc chan proxy.Proxy, wg *sync.WaitGroup) {
	defer wg.Done()
	nodes := c.Get()
	log.Infoln("STATISTIC: Custom \tcount=%d\texec=%s %s", len(nodes), c.Exec, c.Args)
	for _, node := range nodes {
		pc <- node
	}
}

func NewCustomPlugin(options tool.Options) (getter Getter, err error) {
	execInterface, found := options["exec"]
	if found {
		bin, err := AssertTypeStringNotNull(execInterface)
		if err != nil {
			return nil, err
		}
		args, _ := options["args"].(string)
		return &CustomPlugin{Exec: bin, Args: args}, nil
	}
	return nil, ErrorUrlNotFound
}
