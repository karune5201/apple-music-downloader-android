package app

import (
	"os"
	"regexp"

	"github.com/fatih/color"

	"amdl/internal/config"
	"amdl/internal/logx"
)

var (
	forbiddenNames = regexp.MustCompile(`[/\\<>:"|?*]`)
)

type Flags struct {
	Atmos          bool
	AAC            bool
	Select         bool
	ArtistSelect   bool
	Debug          bool
	Verbose        bool
	UILanguage     string
	PrintJSON      bool
	SaveM3U8       bool
	LiteServerFlag string
	Version        bool
	Update         bool
	CheckUpdate    bool
	Yes            bool
	ForceUpdate    bool
	MigrateConfig  bool
}

type State struct {
	Counter     Counter
	OKDict      map[string][]int
	AddedTracks []AddedTrack
}

type Runner struct {
	Config config.ConfigSet
	Flags  Flags
	State  State
}

func NewRunner(cfg config.ConfigSet) *Runner {
	return &Runner{Config: cfg, State: State{OKDict: make(map[string][]int)}}
}

type AddedTrack struct {
	Path     string `json:"path"`
	Artist   string `json:"artist"`
	ArtistID string `json:"artist_id"`
	Album    string `json:"album"`
	Song     string `json:"song"`
}

// topLevelKeys returns the set of top-level YAML keys in data.

// contains reports whether item is present in slice.
func contains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func (r *Runner) LimitString(s string) string {
	if len([]rune(s)) > r.Config.Paths.LimitMax {
		return string([]rune(s)[:r.Config.Paths.LimitMax])
	}
	return s
}

func fileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// askForInput 在等待 stdin 之前先发一行 ##prompt 协议标记。
//
// 标记是语言无关的：Android 端看到它就弹出输入/选择界面；
// 人看到的仍是紧接着的那行翻译好的提示（只带颜色，不加级别标记）。
func askForInput(promptColor *color.Color, prompt string) {
	logx.Print("##prompt\n")
	promptColor.Print(prompt)
}
