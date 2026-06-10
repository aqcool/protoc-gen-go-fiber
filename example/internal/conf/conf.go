// Package conf 加载 example 项目的 YAML 配置（服务端地址与客户端 endpoint）。
package conf

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Bootstrap struct {
	Server Server `yaml:"server"`
	Client Client `yaml:"client"`
}

type Server struct {
	HTTP HTTP `yaml:"http"`
}

type Client struct {
	HTTP HTTP `yaml:"http"`
}

type HTTP struct {
	Addr     string `yaml:"addr"`
	Endpoint string `yaml:"endpoint"`
}

func Load(path string) (*Bootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bc Bootstrap
	if err := yaml.Unmarshal(data, &bc); err != nil {
		return nil, err
	}
	if bc.Server.HTTP.Addr == "" {
		return nil, fmt.Errorf("conf: server.http.addr is required")
	}
	if bc.Client.HTTP.Endpoint == "" {
		return nil, fmt.Errorf("conf: client.http.endpoint is required")
	}
	return &bc, nil
}
