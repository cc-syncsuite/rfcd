package main

import (
	"os"
	"strings"
	"surmc"
)

type rfcdConfig struct {
	BindAddr       string
	Port           int
	Verbosity      int
	Delimiter      string
	Separator      string
	CommandConfigs []CommandConfig
	parsed         map[string]Command
	debug          surmc.Debug
}

func (c *rfcdConfig) GetSeparatorChar() byte { return c.Separator[0] }

func (c *rfcdConfig) GetDelimiterChar() byte { return c.Delimiter[0] }

func (c *rfcdConfig) getCommandConfig(keyword string) (*CommandConfig, os.Error) {
	for _, cc := range c.CommandConfigs {
		if cc.CommandName == keyword {
			return &cc, nil
		}
	}
	return nil, os.NewError("Not a valid key")
}

func (c *rfcdConfig) RegisterCommand(keyword string, fp CommandFunc) {
	if c.parsed == nil {
		c.parsed = make(map[string]Command)
	}
	cc, _ := c.getCommandConfig(keyword)
	opts := stringSliceToMap(cc.CommandParams, ":")
	if c.Verbosity >= 4 {
		globalConfig.debug.DebugPrintf(4, "\t\"%s\" opts:", keyword)
		for key, val := range opts {
			globalConfig.debug.DebugPrintf(4, "\t\t\"%s\" => \"%s\"", key, val)
		}
	}
	c.parsed[strings.ToLower(keyword)] = Command{keyword, fp, opts}
}

func (c *rfcdConfig) GetCommand(keyword string) (cmd Command, ok bool) {
	cmd, ok = c.parsed[strings.ToLower(keyword)]
	return
}

type CommandConfig struct {
	CommandName   string
	CommandParams []string
}

type Command struct {
	cmd      string
	fp       CommandFunc
	confopts map[string]string
}

type CommandFunc func(argv []string, confopts map[string]string) ([]string, os.Error)
