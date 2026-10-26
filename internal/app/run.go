package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"

	ampapi "amdl/internal/amp-api"
	"amdl/internal/config"
	"amdl/internal/download"
	"amdl/internal/i18n"
	"amdl/internal/logx"
	"amdl/internal/updater"
	"amdl/internal/version"
)

func Main() {
	r := NewRunner(config.ConfigSet{})

	var (
		configFile  string
		search_type string
	)
	pflag.StringVarP(&configFile, "config", "c", "", "Path to custom config file (default: config.yaml)")
	pflag.StringVar(&search_type, "search", "", "Search for 'album', 'song', or 'artist'. Provide query after flags.")
	pflag.BoolVar(&r.Flags.Atmos, "atmos", false, "Enable atmos download mode")
	pflag.BoolVar(&r.Flags.AAC, "aac", false, "Enable adm-aac download mode")
	pflag.BoolVar(&r.Flags.Select, "select", false, "Enable selective download")
	pflag.BoolVar(&r.Flags.ArtistSelect, "all-album", false, "Download all artist albums")
	pflag.BoolVar(&r.Flags.Debug, "debug", false, "Enable debug mode to show audio quality information")
	pflag.BoolVar(&r.Flags.Verbose, "verbose", false, "Print verbose detail lines")
	pflag.StringVar(&r.Flags.UILanguage, "ui-language", "", "UI language for logs and messages: zh, ja, en (default: general.ui-language)")
	pflag.BoolVar(&r.Flags.PrintJSON, "json", false, "Output JSON summary at the end")
	pflag.BoolVar(&r.Flags.SaveM3U8, "save-m3u8-playlist", false, "Save M3U8 playlist file")
	pflag.StringVar(&r.Flags.LiteServerFlag, "lite-server", "", "wrapper-lite HTTP API endpoint for this run")
	pflag.Int("alac-max", 0, "Specify the max quality for download alac")
	pflag.Int("atmos-max", 0, "Specify the max quality for download atmos")
	pflag.String("aac-type", "", "Select AAC type, aac aac-binaural aac-downmix")
	pflag.String("mv-audio-type", "", "Select MV audio type, atmos ac3 aac")
	pflag.Int("mv-max", 0, "Specify the max quality for download MV")
	pflag.BoolVarP(&r.Flags.Version, "version", "v", false, "Print version information and exit")
	pflag.BoolVarP(&r.Flags.Update, "update", "U", false, "Perform self-update and interactive config migration")
	pflag.BoolVar(&r.Flags.CheckUpdate, "check-update", false, "Check for available updates without downloading")
	pflag.BoolVarP(&r.Flags.Yes, "yes", "y", false, "Automatically accept defaults during update")
	pflag.BoolVar(&r.Flags.ForceUpdate, "force-update", false, "With --update, reinstall the latest release even if already up to date")
	// Invoked by --update on the freshly installed binary, so migration uses that release's config.yaml.example.
	pflag.BoolVar(&r.Flags.MigrateConfig, "migrate-config", false, "Add options introduced by this version to the config file")
	_ = pflag.CommandLine.MarkHidden("migrate-config")

	pflag.Usage = func() {
		prog := progName()
		logx.PlainT("app.flow.usage", prog)
		logx.PlainT("app.flow.usage_search", prog)
		logx.PlainT("app.flow.usage_options")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	// 语言与日志级别必须在任何输出之前定下来：
	// --ui-language > general.ui-language > AMDL_LANG。
	i18n.Set(firstNonEmpty(r.Flags.UILanguage, r.Config.General.UILanguage, os.Getenv("AMDL_LANG")))
	logx.SetVerbose(r.Flags.Verbose)

	if r.Flags.Version {
		logx.Println(version.Info())
		return
	}

	if r.Flags.MigrateConfig {
		if err := updater.RunConfigMigration(configFile, r.Flags.Yes); err != nil {
			logx.ErrorT("app.flow.migrate_config_failed", err)
			os.Exit(1)
		}
		return
	}

	if r.Flags.CheckUpdate || r.Flags.Update {
		_ = r.loadConfig(config.LoadOptions{
			ConfigFile:             configFile,
			FlagSet:                pflag.CommandLine,
			DisableMissingWarnings: true,
		})
		r.resolveUILanguage()
		proxyURL := r.Config.General.Proxy

		if r.Flags.CheckUpdate {
			if err := updater.CheckUpdateOnly(proxyURL); err != nil {
				logx.WarnT("app.flow.update_check_failed", err)
			}
			return
		}

		if r.Flags.Update {
			if err := updater.ExecuteSelfUpdate(configFile, proxyURL, r.Flags.Yes, r.Flags.ForceUpdate); err != nil {
				logx.ErrorT("app.flow.self_update_failed", err)
				os.Exit(1)
			}
			return
		}
	}

	err := r.loadConfig(config.LoadOptions{
		ConfigFile: configFile,
		FlagSet:    pflag.CommandLine,
	})
	if err != nil {
		logx.ErrorT("app.flow.load_config_failed", err)
		return
	}

	// 配置里的 ui-language 加载后才可见，命令行为空时按配置/环境变量再定一次。
	r.resolveUILanguage()

	updater.PrintStartupUpdateNotice(r.Config.General.Proxy)
	if r.Flags.LiteServerFlag == "" {
		r.Flags.LiteServerFlag = r.Config.General.LiteServer
	}

	if regions, err := r.getLiteRegions(); err != nil {
		logx.WarnT("app.flow.lite_status_failed", err)
	} else {
		logx.DetailT("app.flow.lite_regions", strings.Join(regions, ", "))
	}
	if err := download.Init(r.Config.General.Proxy); err != nil {
		logx.ErrorT("app.flow.proxy_config_failed", err)
		return
	}
	token, err := ampapi.GetToken()
	if err != nil {
		if r.Config.General.AuthorizationToken != "" && r.Config.General.AuthorizationToken != "your-authorization-token" {
			token = strings.Replace(r.Config.General.AuthorizationToken, "Bearer ", "", -1)
		} else {
			logx.ErrorT("app.flow.token_failed", err)
			return
		}
	}

	args := pflag.Args()

	if search_type != "" {
		if len(args) == 0 {
			logx.ErrorT("app.flow.search_query_required")
			pflag.Usage()
			return
		}
		selectedUrl, err := r.handleSearch(search_type, args, token)
		if err != nil {
			logx.ErrorT("app.flow.search_failed", err)
			return
		}
		if selectedUrl == "" {
			logx.PlainT("app.flow.exiting")
			return
		}
		os.Args = []string{selectedUrl}
	} else {
		if len(args) == 0 {
			logx.ErrorT("app.flow.no_urls")
			pflag.Usage()
			return
		}
		os.Args = args
	}

	if strings.Contains(os.Args[0], "/artist/") {
		urlArtistName, urlArtistID, err := r.getUrlArtistName(os.Args[0], token)
		if err != nil {
			logx.ErrorT("app.flow.artist_name_failed", err)
			return
		}
		r.Config.Paths.ArtistFolder = strings.NewReplacer(
			"{UrlArtistName}", r.LimitString(urlArtistName),
			"{ArtistId}", urlArtistID,
		).Replace(r.Config.Paths.ArtistFolder)
		albumArgs, err := r.checkArtist(os.Args[0], token, "albums")
		if err != nil {
			logx.ErrorT("app.flow.artist_albums_failed", err)
			return
		}
		mvArgs, err := r.checkArtist(os.Args[0], token, "music-videos")
		if err != nil {
			logx.WarnT("app.flow.artist_mvs_failed", err)
		}
		os.Args = append(albumArgs, mvArgs...)
	}
	albumTotal := len(os.Args)
	for {
		for albumNum, urlRaw := range os.Args {
			logx.StepT("app.flow.queue", albumNum+1, albumTotal)
			var storefront, albumId string

			if strings.Contains(urlRaw, "/music-video/") {
				logx.DetailT("app.flow.item_mv")
				if r.Flags.Debug {
					continue
				}
				r.State.Counter.Total++
				if r.Config.General.LiteServer == "" {
					logx.WarnT("app.flow.lite_server_unset_mv")
					r.State.Counter.Success++
					continue
				}
				mvSaveDir := strings.NewReplacer(
					"{ArtistName}", "",
					"{UrlArtistName}", "",
					"{ArtistId}", "",
				).Replace(r.Config.Paths.ArtistFolder)
				if mvSaveDir != "" {
					mvSaveDir = filepath.Join(r.Config.Paths.MV, forbiddenNames.ReplaceAllString(mvSaveDir, "_"))
				} else {
					mvSaveDir = r.Config.Paths.MV
				}
				storefront, albumId = checkUrl(urlRaw, "mv")
				err := r.mvDownloader(albumId, mvSaveDir, token, storefront, nil)
				if err != nil {
					logx.WarnT("app.flow.mv_failed", err)
					r.State.Counter.Error++
					continue
				}
				r.State.Counter.Success++
				continue
			}
			if strings.Contains(urlRaw, "/song/") {
				logx.DetailT("app.flow.item_song")
				storefront, songId := checkUrl(urlRaw, "song")
				if storefront == "" || songId == "" {
					logx.WarnT("app.flow.song_url_invalid")
					continue
				}
				err := r.ripSong(songId, token, storefront, r.Config.General.MediaUserToken)
				if err != nil {
					logx.ErrorT("app.flow.rip_song_failed", err)
				}
				continue
			}
			parse, err := url.Parse(urlRaw)
			if err != nil {
				logx.ErrorT("app.flow.url_invalid", err)
				r.State.Counter.Error++
				continue
			}
			var urlArg_i = parse.Query().Get("i")

			if strings.Contains(urlRaw, "/album/") {
				logx.DetailT("app.flow.item_album")
				storefront, albumId = checkUrl(urlRaw, "album")
				err := r.ripAlbum(albumId, token, storefront, r.Config.General.MediaUserToken, urlArg_i)
				if err != nil {
					logx.ErrorT("app.flow.rip_album_failed", err)
				}
			} else if strings.Contains(urlRaw, "/playlist/") {
				logx.DetailT("app.flow.item_playlist")
				storefront, albumId = checkUrl(urlRaw, "playlist")
				err := r.ripPlaylist(albumId, token, storefront, r.Config.General.MediaUserToken)
				if err != nil {
					logx.ErrorT("app.flow.rip_playlist_failed", err)
				}
			} else if strings.Contains(urlRaw, "/station/") {
				logx.DetailT("app.flow.item_station")
				storefront, albumId = checkUrl(urlRaw, "station")
				if len(r.Config.General.MediaUserToken) <= 50 {
					logx.WarnT("app.flow.media_user_token_unset_station")
					continue
				}
				err := r.ripStation(albumId, token, storefront, r.Config.General.MediaUserToken)
				if err != nil {
					logx.ErrorT("app.flow.rip_station_failed", err)
				}
			} else {
				logx.WarnT("app.flow.item_type_unknown")
			}
		}
		logx.OKT("app.flow.summary", r.State.Counter.Success, r.State.Counter.Total, r.State.Counter.Unavailable+r.State.Counter.NotSong, r.State.Counter.Error)
		// 语言无关的汇总行，App 只认这一行的字段名与顺序。
		logx.Print(fmt.Sprintf("##ok=%d total=%d warn=%d err=%d\n",
			r.State.Counter.Success, r.State.Counter.Total,
			r.State.Counter.Unavailable+r.State.Counter.NotSong, r.State.Counter.Error))
		if r.State.Counter.Error == 0 {
			break
		} else if r.Config.General.ExitOnError {
			logx.ErrorT("app.flow.error_exit")
			os.Exit(1)
		} else {
			logx.Print("##prompt\n")
			logx.WarnT("app.flow.retry_prompt")
			fmt.Scanln()
			logx.StepT("app.flow.retry_start")
		}

		r.State.Counter = Counter{}
	}

	// Print JSON output
	if r.Flags.PrintJSON {
		jsonOutput, err := json.Marshal(r.State.AddedTracks)
		if err != nil {
			logx.ErrorT("app.flow.json_failed", err)
		} else {
			logx.Println(string(jsonOutput))
		}
	}
}

// firstNonEmpty 返回第一个非空（去除空白后）的字符串，全部为空时返回空串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// resolveUILanguage 在配置加载完成后按同样优先级再定一次语言：
// --ui-language > general.ui-language > AMDL_LANG。
func (r *Runner) resolveUILanguage() {
	if r.Flags.UILanguage != "" {
		i18n.Set(r.Flags.UILanguage)
		return
	}
	i18n.Set(firstNonEmpty(r.Config.General.UILanguage, os.Getenv("AMDL_LANG")))
}

func getProgName(arg0 string) string {
	if arg0 == "" {
		return "amdl"
	}
	if strings.Contains(arg0, "go-build") {
		return "go run main.go"
	}
	// Normalize backslashes to forward slashes so Windows paths work across platforms (e.g. Linux CI).
	s := strings.ReplaceAll(arg0, "\\", "/")
	s = strings.TrimRight(s, "/")
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		s = s[idx+1:]
	}
	// Strip Windows volume/drive prefix if present (e.g. C:amdl.exe)
	if len(s) >= 2 && s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		s = s[2:]
	}
	if s != "" && s != "." && s != "/" {
		return s
	}
	return "amdl"
}

func progName() string {
	if len(os.Args) > 0 {
		return getProgName(os.Args[0])
	}
	return "amdl"
}
