package app

import (
	"amdl/internal/amp-api"
	fairplayrip "amdl/internal/fairplay-rip"
	"amdl/internal/logx"
	"amdl/internal/media/alacfix"
	defrag "amdl/internal/media/defrag"
	"amdl/internal/media/lyrics"
	"amdl/internal/model"
	"amdl/internal/widevine-rip"
	"amdl/internal/widevine-rip/runv5"
	"context"
	"errors"
	"fmt"
	"github.com/itouakirai/go-mp4tag"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type lyricsResult struct {
	lrc string
	err error
}

func (r *Runner) ripTrack(track *model.Track, token string, mediaUserToken string) {
	var err error
	started := time.Now()
	r.State.Counter.Total++
	logx.StepT("app.rip.track_banner", track.TaskNum, track.TaskTotal, track.Type)

	//mv dl dev
	if track.Type == "music-videos" {
		if r.Config.General.LiteServer == "" {
			logx.WarnT("app.rip.mv_no_lite_server")
			r.State.Counter.Success++
			return
		}
		err := r.mvDownloader(track.ID, track.SaveDir, token, track.Storefront, track)
		if err != nil {
			logx.ErrorT("app.rip.mv_download_failed", err)
			r.State.Counter.Error++
			return
		}
		r.State.Counter.Success++
		return
	}

	needDlAacLc := false
	if r.Flags.AAC && r.Config.Media.AacType == "aac-lc" {
		needDlAacLc = true
	}
	if track.WebM3u8 == "" && !needDlAacLc {
		if r.Flags.Atmos {
			logx.WarnT("app.rip.unavailable")
			r.State.Counter.Unavailable++
			return
		}
		logx.WarnT("app.rip.unavailable_try_aac")
		needDlAacLc = true
	}
	needCheck := false

	if r.Config.Media.GetM3u8Mode == "all" {
		needCheck = true
	} else if r.Config.Media.GetM3u8Mode == "hires" && contains(track.Resp.Attributes.AudioTraits, "hi-res-lossless") {
		needCheck = true
	}
	var EnhancedHls_m3u8 string
	if needCheck && !needDlAacLc {
		EnhancedHls_m3u8, _ = r.checkM3u8(track.ID, "song")
		if strings.HasSuffix(EnhancedHls_m3u8, ".m3u8") {
			track.DeviceM3u8 = EnhancedHls_m3u8
			track.M3u8 = EnhancedHls_m3u8
		}
	}
	var Quality string
	if strings.Contains(r.Config.Paths.SongFile, "Quality") {
		if r.Flags.Atmos {
			Quality = fmt.Sprintf("%dKbps", r.Config.Media.AtmosMax-2000)
		} else if needDlAacLc {
			Quality = "256Kbps"
		} else {
			_, Quality, err = r.extractMedia(track.M3u8, true)
			if err != nil {
				logx.ErrorT("app.rip.quality_extract_failed", err)
				r.State.Counter.Error++
				return
			}
		}
	}
	track.Quality = Quality
	logx.DebugT("app.rip.track_params", logx.KV("quality", Quality), logx.KV("codec", track.Codec))
	logx.DebugT("app.rip.track_dir", logx.KV("dir", track.SaveDir))
	logx.DebugT("app.rip.track_manifest", logx.KV("m3u8", track.M3u8))

	stringsToJoin := []string{}
	if track.Resp.Attributes.IsAppleDigitalMaster {
		if r.Config.Paths.AppleMaster != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.AppleMaster)
		}
	}
	if track.Resp.Attributes.ContentRating == "explicit" {
		if r.Config.Paths.Explicit != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.Explicit)
		}
	}
	if track.Resp.Attributes.ContentRating == "clean" {
		if r.Config.Paths.Clean != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.Clean)
		}
	}
	Tag_string := strings.Join(stringsToJoin, " ")

	songName := strings.NewReplacer(
		"{SongId}", track.ID,
		"{SongNumer}", fmt.Sprintf("%02d", track.TaskNum),
		"{ArtistName}", r.LimitString(track.Resp.Attributes.ArtistName),
		"{SongName}", r.LimitString(track.Resp.Attributes.Name),
		"{DiscNumber}", fmt.Sprintf("%0d", track.Resp.Attributes.DiscNumber),
		"{TrackNumber}", fmt.Sprintf("%0d", track.Resp.Attributes.TrackNumber),
		"{Quality}", Quality,
		"{Tag}", Tag_string,
		"{Codec}", track.Codec,
	).Replace(r.Config.Paths.SongFile)
	logx.DetailT("app.rip.song_file", songName)
	filename := fmt.Sprintf("%s.m4a", forbiddenNames.ReplaceAllString(songName, "_"))
	track.SaveName = filename
	trackPath := filepath.Join(track.SaveDir, track.SaveName)
	lrcFilename := fmt.Sprintf("%s.%s", forbiddenNames.ReplaceAllString(songName, "_"), r.Config.Metadata.Lyrics.Format)

	// Determine possible post-conversion target file (so we can skip re-download)
	var convertedPath string
	considerConverted := false
	if r.Config.Convert.AfterDownload &&
		r.Config.Convert.Format != "" &&
		strings.ToLower(r.Config.Convert.Format) != "copy" &&
		!r.Config.Convert.KeepOriginal {
		convertedPath = strings.TrimSuffix(trackPath, filepath.Ext(trackPath)) + "." + strings.ToLower(r.Config.Convert.Format)
		considerConverted = true
	}
	// Existence check now considers converted output (if original was deleted)
	existsOriginal, err := fileExists(trackPath)
	if err != nil {
		logx.WarnT("app.rip.exists_check_failed", err)
	}
	if existsOriginal {
		logx.OKT("app.rip.exists")
		r.State.Counter.Success++
		r.State.OKDict[track.PreID] = append(r.State.OKDict[track.PreID], track.TaskNum)

		tArtistId := ""
		if len(track.Resp.Relationships.Artists.Data) > 0 {
			tArtistId = track.Resp.Relationships.Artists.Data[0].ID
		}
		r.State.AddedTracks = append(r.State.AddedTracks, AddedTrack{
			Path:     trackPath,
			Artist:   track.Resp.Attributes.ArtistName,
			ArtistID: tArtistId,
			Album:    track.Resp.Attributes.AlbumName,
			Song:     track.Resp.Attributes.Name,
		})
		return
	}
	if considerConverted {
		existsConverted, err2 := fileExists(convertedPath)
		if err2 == nil && existsConverted {
			logx.OKT("app.rip.exists_converted")
			r.State.Counter.Success++
			r.State.OKDict[track.PreID] = append(r.State.OKDict[track.PreID], track.TaskNum)

			tArtistId := ""
			if len(track.Resp.Relationships.Artists.Data) > 0 {
				tArtistId = track.Resp.Relationships.Artists.Data[0].ID
			}
			r.State.AddedTracks = append(r.State.AddedTracks, AddedTrack{
				Path:     convertedPath,
				Artist:   track.Resp.Attributes.ArtistName,
				ArtistID: tArtistId,
				Album:    track.Resp.Attributes.AlbumName,
				Song:     track.Resp.Attributes.Name,
			})
			return
		}
	}

	//提前获取到的播放列表下track所在的专辑信息
	if track.PreType == "playlists" && r.Config.Metadata.Tags.UseSongInfoForPlaylist {
		if err := track.GetAlbumData(token); err != nil {
			logx.ErrorT("app.rip.album_data_failed", err)
			r.State.Counter.Error++
			return
		}
	}

	// Fetch lyrics concurrently with the audio download; the result (and any
	// message) is handled once the download is done, or on early return.
	var lrc string
	var lyricsCh chan lyricsResult
	if r.Config.Metadata.Lyrics.Embed || r.Config.Metadata.Lyrics.SaveFile {
		lyricsCh = make(chan lyricsResult, 1)
		go func() {
			lrcStr, err := lyrics.Get(track.ID, r.Config.Metadata.Lyrics.Type, r.Config.General.Language, r.Config.Metadata.Lyrics.Format, r.Config.General.LiteServer, r.Config.Metadata.Lyrics.Extra)
			lyricsCh <- lyricsResult{lrc: lrcStr, err: err}
		}()
	}
	collectLyrics := func() {
		if lyricsCh == nil {
			return
		}
		res := <-lyricsCh
		lyricsCh = nil
		if res.err != nil {
			if errors.Is(res.err, lyrics.ErrLyricsNotFound) {
				logx.WarnT("app.rip.lyrics_unavailable")
			} else {
				logx.WarnT("app.rip.lyrics_failed", res.err)
			}
			return
		}
		if r.Config.Metadata.Lyrics.SaveFile && res.lrc != "" {
			if err := r.writeLyrics(track.SaveDir, lrcFilename, res.lrc); err != nil {
				logx.WarnT("app.rip.lyrics_write_failed", err)
			}
		}
		if r.Config.Metadata.Lyrics.Embed && res.lrc != "" {
			lrc = res.lrc
		}
	}
	defer collectLyrics()

	if needDlAacLc {
		if r.Config.General.LiteServer == "" {
			logx.ErrorT("app.rip.aac_lc_need_lite_server")
			r.State.Counter.Error++
			return
		}
		err := runv5.DownloadSong(context.Background(), track.ID, trackPath, r.Config.General.LiteServer)
		if err != nil {
			logx.ErrorT("app.rip.aac_lc_download_failed", err)
			if err.Error() == "Unavailable" {
				r.State.Counter.Unavailable++
				return
			}
			r.State.Counter.Error++
			return
		}
	} else {
		trackM3u8Url, _, err := r.extractMedia(track.M3u8, false)
		if err != nil {
			logx.ErrorT("app.rip.manifest_extract_failed", err)
			r.State.Counter.Unavailable++
			return
		}
		//边下载边解密
		//wrapper-lite 模板解密
		err = fairplayrip.Run(track.ID, trackM3u8Url, trackPath, r.Config)
		if err != nil {
			logx.ErrorT("app.rip.fairplay_failed", err)
			r.State.Counter.Error++
			return
		}

	}
	collectLyrics()

	// 将 fMP4 解碎片为普通 MP4；元数据和封面统一交给后续 writeMP4Tags 写入。
	removeCoverAfterWrite := false
	if r.Config.Metadata.Artwork.Embed {
		if (strings.Contains(track.PreID, "pl.") || strings.Contains(track.PreID, "ra.")) && r.Config.Metadata.Artwork.DlForPlaylist {
			track.CoverPath, err = r.writeCover(track.SaveDir, track.ID, track.Resp.Attributes.Artwork.URL)
			if err != nil {
				logx.WarnT("app.rip.cover_write_failed", err)
			} else {
				removeCoverAfterWrite = true
			}
		}
	}

	if err := defrag.DefragmentMP4(trackPath); err != nil {
		logx.ErrorT("app.rip.defrag_failed", err)
		r.State.Counter.Error++
		return
	}
	track.SavePath = trackPath

	if r.Config.Media.ALACFix {
		err = alacfix.Run(track.SavePath, false)
		if err != nil {
			logx.ErrorT("app.rip.alac_fix_failed", err)
			r.State.Counter.Unavailable++
			return
		}
	}

	err = r.writeMP4Tags(track, lrc)
	if err != nil {
		logx.ErrorT("app.rip.tags_write_failed", err)
		r.State.Counter.Unavailable++
		return
	}
	if removeCoverAfterWrite {
		if err := os.Remove(track.CoverPath); err != nil {
			logx.ErrorT("app.rip.cover_delete_failed", track.CoverPath)
			r.State.Counter.Error++
			return
		}
	}

	var writtenBytes int64
	if st, statErr := os.Stat(trackPath); statErr == nil {
		writtenBytes = st.Size()
	}
	logx.DetailT("app.rip.track_written", track.SaveName, logx.Bytes(writtenBytes), logx.Since(started))

	// CONVERSION FEATURE hook
	r.convertIfNeeded(track)

	tArtistId := ""
	if len(track.Resp.Relationships.Artists.Data) > 0 {
		tArtistId = track.Resp.Relationships.Artists.Data[0].ID
	}
	r.State.AddedTracks = append(r.State.AddedTracks, AddedTrack{
		Path:     track.SavePath,
		Artist:   track.Resp.Attributes.ArtistName,
		ArtistID: tArtistId,
		Album:    track.Resp.Attributes.AlbumName,
		Song:     track.Resp.Attributes.Name,
	})

	r.State.Counter.Success++
	r.State.OKDict[track.PreID] = append(r.State.OKDict[track.PreID], track.TaskNum)
}

func releaseYear(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

func (r *Runner) ripStation(albumId string, token string, storefront string, mediaUserToken string) error {
	station := model.NewStation(storefront, albumId)
	err := station.GetResp(mediaUserToken, token, r.Config.General.Language)
	if err != nil {
		return err
	}
	logx.DetailT("app.rip.station_type", station.Type)
	meta := station.Resp

	var Codec string
	if r.Flags.Atmos {
		Codec = "ATMOS"
	} else if r.Flags.AAC {
		Codec = "AAC"
	} else {
		Codec = "ALAC"
	}
	station.Codec = Codec
	logx.DebugT("app.rip.codec", logx.KV("codec", Codec))
	var singerFoldername string
	if r.Config.Paths.ArtistFolder != "" {
		singerFoldername = strings.NewReplacer(
			"{ArtistName}", "Apple Music Station",
			"{ArtistId}", "",
			"{UrlArtistName}", "Apple Music Station",
		).Replace(r.Config.Paths.ArtistFolder)
		if strings.HasSuffix(singerFoldername, ".") {
			singerFoldername = strings.ReplaceAll(singerFoldername, ".", "")
		}
		singerFoldername = strings.TrimSpace(singerFoldername)
		logx.DetailT("app.rip.artist_folder", singerFoldername)
	}
	singerFolder := filepath.Join(r.Config.Paths.Alac, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	if r.Flags.Atmos {
		singerFolder = filepath.Join(r.Config.Paths.Atmos, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	}
	if r.Flags.AAC {
		singerFolder = filepath.Join(r.Config.Paths.Aac, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	}
	if err := createDirectory(singerFolder); err != nil {
		return err
	}
	station.SaveDir = singerFolder

	playlistFolder := strings.NewReplacer(
		"{ArtistName}", "Apple Music Station",
		"{PlaylistName}", r.LimitString(station.Name),
		"{PlaylistId}", station.ID,
		"{Quality}", "",
		"{Codec}", Codec,
		"{Tag}", "",
	).Replace(r.Config.Paths.PlaylistFolder)
	playlistFolderPath, err := r.prepareCollectionFolder(singerFolder, playlistFolder)
	if err != nil {
		return err
	}
	station.SaveName = playlistFolder
	logx.DetailT("app.rip.playlist_folder", playlistFolder)

	covPath, err := r.writeCover(playlistFolderPath, "cover", meta.Data[0].Attributes.Artwork.URL)
	if err != nil {
		logx.WarnT("app.rip.cover_write_failed", err)
	}
	station.CoverPath = covPath

	if r.Config.Metadata.Artwork.SaveAnimated {
		r.saveAnimatedArtwork(playlistFolderPath, meta.Data[0].Attributes.EditorialVideo.MotionSquare.Video, "")
	}
	if station.Type == "stream" {
		startIdx := len(r.State.AddedTracks)
		r.State.Counter.Total++
		if contains(r.State.OKDict[station.ID], 1) {
			r.State.Counter.Success++
			return nil
		}
		songName := strings.NewReplacer(
			"{SongId}", station.ID,
			"{SongNumer}", "01",
			"{SongName}", r.LimitString(station.Name),
			"{DiscNumber}", "1",
			"{TrackNumber}", "1",
			"{Quality}", "256Kbps",
			"{Tag}", "",
			"{Codec}", "AAC",
		).Replace(r.Config.Paths.SongFile)
		logx.DetailT("app.rip.song_file", songName)
		trackPath := filepath.Join(playlistFolderPath, fmt.Sprintf("%s.m4a", forbiddenNames.ReplaceAllString(songName, "_")))
		exists, _ := fileExists(trackPath)
		if exists {
			r.State.Counter.Success++
			r.State.OKDict[station.ID] = append(r.State.OKDict[station.ID], 1)

			logx.OKT("app.rip.station_exists")
			r.State.AddedTracks = append(r.State.AddedTracks, AddedTrack{
				Path:     trackPath,
				Artist:   "Apple Music Station",
				ArtistID: "",
				Album:    station.Name,
				Song:     station.Name,
			})
			r.saveM3UPlaylist(playlistFolderPath, playlistFolder, startIdx)
			return nil
		}
		assetsUrl, serverUrl, err := ampapi.GetStationAssetsUrlAndServerUrl(station.ID, mediaUserToken, token)
		if err != nil {
			logx.ErrorT("app.rip.station_assets_failed", err)
			r.State.Counter.Error++
			return err
		}
		creds := widevinerip.Credentials{AuthToken: token, MediaUserToken: mediaUserToken}
		trackM3U8, err := widevinerip.ResolveStationVariantPlaylist(assetsUrl, creds)
		if err != nil {
			logx.ErrorT("app.rip.station_playlist_failed", err)
			r.State.Counter.Error++
			return err
		}
		ctx := context.Background()
		stream, err := widevinerip.FetchStationStream(ctx, station.ID, trackM3U8, creds, serverUrl)
		if err != nil {
			logx.ErrorT("app.rip.station_key_failed", err)
			r.State.Counter.Error++
			return err
		}
		err = widevinerip.DownloadStream(ctx, stream, trackPath)
		if err != nil {
			_ = os.Remove(trackPath)
			logx.ErrorT("app.rip.station_download_failed", err)
			r.State.Counter.Error++
			return err
		}
		if err := defrag.DefragmentMP4(trackPath); err != nil {
			_ = os.Remove(trackPath)
			logx.ErrorT("app.rip.defrag_failed", err)
			r.State.Counter.Error++
			return err
		}
		tags := &mp4tag.MP4Tags{
			Title:       station.Name,
			Artist:      "Apple Music Station",
			Album:       station.Name,
			AlbumArtist: "Apple Music Station",
			TrackNumber: 1,
			TrackTotal:  1,
			DiscNumber:  1,
			DiscTotal:   1,
			Custom: map[string]string{
				"PERFORMER": "Apple Music Station",
			},
		}
		if r.Config.Metadata.Artwork.Embed && station.CoverPath != "" {
			cover, err := os.ReadFile(station.CoverPath)
			if err != nil {
				_ = os.Remove(trackPath)
				logx.ErrorT("app.rip.station_cover_read_failed", err)
				r.State.Counter.Error++
				return err
			}
			tags.Pictures = []*mp4tag.MP4Picture{{
				Format: mp4tag.ImageTypeAuto,
				Data:   cover,
			}}
		}
		mp4, err := mp4tag.Open(trackPath)
		if err != nil {
			_ = os.Remove(trackPath)
			logx.ErrorT("app.rip.station_tag_open_failed", err)
			r.State.Counter.Error++
			return err
		}
		err = mp4.Write(tags, []string{})
		_ = mp4.Close()
		if err != nil {
			_ = os.Remove(trackPath)
			logx.ErrorT("app.rip.station_tag_write_failed", err)
			r.State.Counter.Error++
			return err
		}
		r.State.AddedTracks = append(r.State.AddedTracks, AddedTrack{
			Path:     trackPath,
			Artist:   "Apple Music Station",
			ArtistID: "",
			Album:    station.Name,
			Song:     station.Name,
		})
		r.State.Counter.Success++
		r.State.OKDict[station.ID] = append(r.State.OKDict[station.ID], 1)
		if st, statErr := os.Stat(trackPath); statErr == nil {
			logx.DetailT("app.rip.station_written", trackPath, logx.Bytes(st.Size()))
		}
		r.saveM3UPlaylist(playlistFolderPath, playlistFolder, startIdx)
		return nil
	}

	for i := range station.Tracks {
		station.Tracks[i].CoverPath = covPath
		station.Tracks[i].SaveDir = playlistFolderPath
		station.Tracks[i].Codec = Codec
	}

	trackTotal := len(station.Tracks)
	arr := make([]int, trackTotal)
	for i := 0; i < trackTotal; i++ {
		arr[i] = i + 1
	}
	var selected []int

	if !r.Flags.Select {
		selected = arr
	} else {
		selected = station.ShowSelect()
	}
	startIdx := len(r.State.AddedTracks)
	for i := range station.Tracks {
		i++
		if contains(selected, i) {
			r.ripTrack(&station.Tracks[i-1], token, mediaUserToken)
		}
	}
	r.saveM3UPlaylist(playlistFolderPath, playlistFolder, startIdx)
	return nil
}

func (r *Runner) ripAlbum(albumId string, token string, storefront string, mediaUserToken string, urlArg_i string) error {
	album := model.NewAlbum(storefront, albumId)
	err := album.GetResp(token, r.Config.General.Language)
	if err != nil {
		logx.ErrorT("app.rip.album_response_failed", err)
		return err
	}
	meta := album.Resp
	if r.Flags.Debug {
		logx.Println(meta.Data[0].Attributes.ArtistName)
		logx.Println(meta.Data[0].Attributes.Name)

		for trackNum, track := range meta.Data[0].Relationships.Tracks.Data {
			trackNum++
			logx.StepT("app.rip.debug_track", trackNum, len(meta.Data[0].Relationships.Tracks.Data))
			logx.Printf("%02d. %s\n", trackNum, track.Attributes.Name)

			manifest, err := ampapi.GetSongResp(storefront, track.ID, album.Language, token)
			if err != nil {
				logx.ErrorT("app.rip.debug_manifest_failed", trackNum, err)
				continue
			}

			var m3u8Url string
			if manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls != "" {
				m3u8Url = manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls
			}
			needCheck := false
			if r.Config.Media.GetM3u8Mode == "all" {
				needCheck = true
			} else if r.Config.Media.GetM3u8Mode == "hires" && contains(track.Attributes.AudioTraits, "hi-res-lossless") {
				needCheck = true
			}
			if needCheck {
				fullM3u8Url, err := r.checkM3u8(track.ID, "song")
				if err == nil && strings.HasSuffix(fullM3u8Url, ".m3u8") {
					m3u8Url = fullM3u8Url
				} else {
					logx.WarnT("app.rip.m3u8_fallback")
				}
			}

			_, _, err = r.extractMedia(m3u8Url, true)
			if err != nil {
				logx.ErrorT("app.rip.debug_quality_failed", trackNum, err)
				continue
			}
		}
		return nil
	}
	var Codec string
	if r.Flags.Atmos {
		Codec = "ATMOS"
	} else if r.Flags.AAC {
		Codec = "AAC"
	} else {
		Codec = "ALAC"
	}
	album.Codec = Codec
	logx.DebugT("app.rip.codec", logx.KV("codec", Codec))
	var singerFoldername string
	if r.Config.Paths.ArtistFolder != "" {
		if len(meta.Data[0].Relationships.Artists.Data) > 0 {
			singerFoldername = strings.NewReplacer(
				"{UrlArtistName}", r.LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistName}", r.LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistId}", meta.Data[0].Relationships.Artists.Data[0].ID,
			).Replace(r.Config.Paths.ArtistFolder)
		} else {
			singerFoldername = strings.NewReplacer(
				"{UrlArtistName}", r.LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistName}", r.LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistId}", "",
			).Replace(r.Config.Paths.ArtistFolder)
		}
		if strings.HasSuffix(singerFoldername, ".") {
			singerFoldername = strings.ReplaceAll(singerFoldername, ".", "")
		}
		singerFoldername = strings.TrimSpace(singerFoldername)
		logx.DetailT("app.rip.artist_folder", singerFoldername)
	}
	singerFolder := filepath.Join(r.Config.Paths.Alac, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	if r.Flags.Atmos {
		singerFolder = filepath.Join(r.Config.Paths.Atmos, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	}
	if r.Flags.AAC {
		singerFolder = filepath.Join(r.Config.Paths.Aac, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	}
	if err := createDirectory(singerFolder); err != nil {
		return err
	}
	album.SaveDir = singerFolder
	var Quality string
	if strings.Contains(r.Config.Paths.AlbumFolder, "Quality") {
		if r.Flags.Atmos {
			Quality = fmt.Sprintf("%dKbps", r.Config.Media.AtmosMax-2000)
		} else if r.Flags.AAC && r.Config.Media.AacType == "aac-lc" {
			Quality = "256Kbps"
		} else {
			manifest1, err := ampapi.GetSongResp(storefront, meta.Data[0].Relationships.Tracks.Data[0].ID, album.Language, token)
			if err != nil {
				logx.ErrorT("app.rip.manifest_get_failed", err)
			} else {
				if manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls == "" {
					Codec = "AAC"
					Quality = "256Kbps"
				} else {
					needCheck := false

					if r.Config.Media.GetM3u8Mode == "all" {
						needCheck = true
					} else if r.Config.Media.GetM3u8Mode == "hires" && contains(meta.Data[0].Relationships.Tracks.Data[0].Attributes.AudioTraits, "hi-res-lossless") {
						needCheck = true
					}
					var EnhancedHls_m3u8 string
					if needCheck {
						EnhancedHls_m3u8, _ = r.checkM3u8(meta.Data[0].Relationships.Tracks.Data[0].ID, "album")
						if strings.HasSuffix(EnhancedHls_m3u8, ".m3u8") {
							manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls = EnhancedHls_m3u8
						}
					}
					_, Quality, err = r.extractMedia(manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls, true)
					if err != nil {
						logx.ErrorT("app.rip.quality_extract_failed", err)
					}
				}
			}
		}
	}
	stringsToJoin := []string{}
	if meta.Data[0].Attributes.IsAppleDigitalMaster || meta.Data[0].Attributes.IsMasteredForItunes {
		if r.Config.Paths.AppleMaster != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.AppleMaster)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "explicit" {
		if r.Config.Paths.Explicit != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.Explicit)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "clean" {
		if r.Config.Paths.Clean != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.Clean)
		}
	}
	Tag_string := strings.Join(stringsToJoin, " ")
	var albumFolderName string
	albumFolderName = strings.NewReplacer(
		"{ReleaseDate}", meta.Data[0].Attributes.ReleaseDate,
		"{ReleaseYear}", releaseYear(meta.Data[0].Attributes.ReleaseDate),
		"{ArtistName}", r.LimitString(meta.Data[0].Attributes.ArtistName),
		"{AlbumName}", r.LimitString(meta.Data[0].Attributes.Name),
		"{UPC}", meta.Data[0].Attributes.Upc,
		"{RecordLabel}", meta.Data[0].Attributes.RecordLabel,
		"{Copyright}", meta.Data[0].Attributes.Copyright,
		"{AlbumId}", albumId,
		"{Quality}", Quality,
		"{Codec}", Codec,
		"{Tag}", Tag_string,
	).Replace(r.Config.Paths.AlbumFolder)

	albumFolderPath, err := r.prepareCollectionFolder(singerFolder, albumFolderName)
	if err != nil {
		return err
	}
	album.SaveName = albumFolderName
	logx.DetailT("app.rip.album_folder", albumFolderName)
	if r.Config.Metadata.Artwork.SaveArtist && len(meta.Data[0].Relationships.Artists.Data) > 0 {
		if meta.Data[0].Relationships.Artists.Data[0].Attributes.Artwork.Url != "" {
			_, err = r.writeCover(singerFolder, "folder", meta.Data[0].Relationships.Artists.Data[0].Attributes.Artwork.Url)
			if err != nil {
				logx.WarnT("app.rip.artist_cover_failed", err)
			}
		}
	}
	covPath, err := r.writeCover(albumFolderPath, "cover", meta.Data[0].Attributes.Artwork.URL)
	if err != nil {
		logx.WarnT("app.rip.cover_write_failed", err)
	}
	if r.Config.Metadata.Artwork.SaveAnimated {
		r.saveAnimatedArtwork(
			albumFolderPath,
			meta.Data[0].Attributes.EditorialVideo.MotionDetailSquare.Video,
			meta.Data[0].Attributes.EditorialVideo.MotionDetailTall.Video,
		)
	}
	for i := range album.Tracks {
		album.Tracks[i].CoverPath = covPath
		album.Tracks[i].SaveDir = albumFolderPath
		album.Tracks[i].Codec = Codec
	}
	trackTotal := len(meta.Data[0].Relationships.Tracks.Data)
	arr := make([]int, trackTotal)
	for i := 0; i < trackTotal; i++ {
		arr[i] = i + 1
	}

	if urlArg_i != "" {
		for i := range album.Tracks {
			if urlArg_i == album.Tracks[i].ID {
				r.ripTrack(&album.Tracks[i], token, mediaUserToken)
				return nil
			}
		}
		return nil
	}
	var selected []int
	if !r.Flags.Select {
		selected = arr
	} else {
		selected = album.ShowSelect()
	}
	startIdx := len(r.State.AddedTracks)
	for i := range album.Tracks {
		i++
		if contains(r.State.OKDict[albumId], i) {
			r.State.Counter.Total++
			r.State.Counter.Success++
			continue
		}
		if contains(selected, i) {
			r.ripTrack(&album.Tracks[i-1], token, mediaUserToken)
		}
	}
	r.saveM3UPlaylist(albumFolderPath, albumFolderName, startIdx)
	return nil

}

func (r *Runner) ripPlaylist(playlistId string, token string, storefront string, mediaUserToken string) error {
	playlist := model.NewPlaylist(storefront, playlistId)
	err := playlist.GetResp(token, r.Config.General.Language)
	if err != nil {
		logx.ErrorT("app.rip.playlist_response_failed", err)
		return err
	}
	meta := playlist.Resp
	if r.Flags.Debug {
		logx.Println(meta.Data[0].Attributes.ArtistName)
		logx.Println(meta.Data[0].Attributes.Name)

		for trackNum, track := range meta.Data[0].Relationships.Tracks.Data {
			trackNum++
			logx.StepT("app.rip.debug_track", trackNum, len(meta.Data[0].Relationships.Tracks.Data))
			logx.Printf("%02d. %s\n", trackNum, track.Attributes.Name)

			manifest, err := ampapi.GetSongResp(storefront, track.ID, playlist.Language, token)
			if err != nil {
				logx.ErrorT("app.rip.debug_manifest_failed", trackNum, err)
				continue
			}

			var m3u8Url string
			if manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls != "" {
				m3u8Url = manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls
			}
			needCheck := false
			if r.Config.Media.GetM3u8Mode == "all" {
				needCheck = true
			} else if r.Config.Media.GetM3u8Mode == "hires" && contains(track.Attributes.AudioTraits, "hi-res-lossless") {
				needCheck = true
			}
			if needCheck {
				fullM3u8Url, err := r.checkM3u8(track.ID, "song")
				if err == nil && strings.HasSuffix(fullM3u8Url, ".m3u8") {
					m3u8Url = fullM3u8Url
				} else {
					logx.WarnT("app.rip.m3u8_fallback")
				}
			}

			_, _, err = r.extractMedia(m3u8Url, true)
			if err != nil {
				logx.ErrorT("app.rip.debug_quality_failed", trackNum, err)
				continue
			}
		}
		return nil
	}
	var Codec string
	if r.Flags.Atmos {
		Codec = "ATMOS"
	} else if r.Flags.AAC {
		Codec = "AAC"
	} else {
		Codec = "ALAC"
	}
	playlist.Codec = Codec
	logx.DebugT("app.rip.codec", logx.KV("codec", Codec))
	var singerFoldername string
	if r.Config.Paths.ArtistFolder != "" {
		singerFoldername = strings.NewReplacer(
			"{ArtistName}", "Apple Music",
			"{ArtistId}", "",
			"{UrlArtistName}", "Apple Music",
		).Replace(r.Config.Paths.ArtistFolder)
		if strings.HasSuffix(singerFoldername, ".") {
			singerFoldername = strings.ReplaceAll(singerFoldername, ".", "")
		}
		singerFoldername = strings.TrimSpace(singerFoldername)
		logx.DetailT("app.rip.artist_folder", singerFoldername)
	}
	singerFolder := filepath.Join(r.Config.Paths.Alac, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	if r.Flags.Atmos {
		singerFolder = filepath.Join(r.Config.Paths.Atmos, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	}
	if r.Flags.AAC {
		singerFolder = filepath.Join(r.Config.Paths.Aac, forbiddenNames.ReplaceAllString(singerFoldername, "_"))
	}
	if err := createDirectory(singerFolder); err != nil {
		return err
	}
	playlist.SaveDir = singerFolder

	var Quality string
	if strings.Contains(r.Config.Paths.AlbumFolder, "Quality") {
		if r.Flags.Atmos {
			Quality = fmt.Sprintf("%dKbps", r.Config.Media.AtmosMax-2000)
		} else if r.Flags.AAC && r.Config.Media.AacType == "aac-lc" {
			Quality = "256Kbps"
		} else {
			manifest1, err := ampapi.GetSongResp(storefront, meta.Data[0].Relationships.Tracks.Data[0].ID, playlist.Language, token)
			if err != nil {
				logx.ErrorT("app.rip.manifest_get_failed", err)
			} else {
				if manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls == "" {
					Codec = "AAC"
					Quality = "256Kbps"
				} else {
					needCheck := false

					if r.Config.Media.GetM3u8Mode == "all" {
						needCheck = true
					} else if r.Config.Media.GetM3u8Mode == "hires" && contains(meta.Data[0].Relationships.Tracks.Data[0].Attributes.AudioTraits, "hi-res-lossless") {
						needCheck = true
					}
					var EnhancedHls_m3u8 string
					if needCheck {
						EnhancedHls_m3u8, _ = r.checkM3u8(meta.Data[0].Relationships.Tracks.Data[0].ID, "album")
						if strings.HasSuffix(EnhancedHls_m3u8, ".m3u8") {
							manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls = EnhancedHls_m3u8
						}
					}
					_, Quality, err = r.extractMedia(manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls, true)
					if err != nil {
						logx.ErrorT("app.rip.quality_extract_failed", err)
					}
				}
			}
		}
	}
	stringsToJoin := []string{}
	if meta.Data[0].Attributes.IsAppleDigitalMaster || meta.Data[0].Attributes.IsMasteredForItunes {
		if r.Config.Paths.AppleMaster != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.AppleMaster)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "explicit" {
		if r.Config.Paths.Explicit != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.Explicit)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "clean" {
		if r.Config.Paths.Clean != "" {
			stringsToJoin = append(stringsToJoin, r.Config.Paths.Clean)
		}
	}
	Tag_string := strings.Join(stringsToJoin, " ")
	playlistFolder := strings.NewReplacer(
		"{ArtistName}", "Apple Music",
		"{PlaylistName}", r.LimitString(meta.Data[0].Attributes.Name),
		"{PlaylistId}", playlistId,
		"{Quality}", Quality,
		"{Codec}", Codec,
		"{Tag}", Tag_string,
	).Replace(r.Config.Paths.PlaylistFolder)
	playlistFolderPath, err := r.prepareCollectionFolder(singerFolder, playlistFolder)
	if err != nil {
		return err
	}
	playlist.SaveName = playlistFolder
	logx.DetailT("app.rip.playlist_folder", playlistFolder)
	covPath, err := r.writeCover(playlistFolderPath, "cover", meta.Data[0].Attributes.Artwork.URL)
	if err != nil {
		logx.WarnT("app.rip.cover_write_failed", err)
	}

	for i := range playlist.Tracks {
		playlist.Tracks[i].CoverPath = covPath
		playlist.Tracks[i].SaveDir = playlistFolderPath
		playlist.Tracks[i].Codec = Codec
	}

	if r.Config.Metadata.Artwork.SaveAnimated {
		r.saveAnimatedArtwork(
			playlistFolderPath,
			meta.Data[0].Attributes.EditorialVideo.MotionDetailSquare.Video,
			meta.Data[0].Attributes.EditorialVideo.MotionDetailTall.Video,
		)
	}
	trackTotal := len(meta.Data[0].Relationships.Tracks.Data)
	arr := make([]int, trackTotal)
	for i := 0; i < trackTotal; i++ {
		arr[i] = i + 1
	}
	var selected []int

	if !r.Flags.Select {
		selected = arr
	} else {
		selected = playlist.ShowSelect()
	}
	startIdx := len(r.State.AddedTracks)
	for i := range playlist.Tracks {
		i++
		if contains(r.State.OKDict[playlistId], i) {
			r.State.Counter.Total++
			r.State.Counter.Success++
			continue
		}
		if contains(selected, i) {
			r.ripTrack(&playlist.Tracks[i-1], token, mediaUserToken)
		}
	}
	r.saveM3UPlaylist(playlistFolderPath, playlistFolder, startIdx)
	return nil
}

func (r *Runner) ripSong(songId string, token string, storefront string, mediaUserToken string) error {
	// Get song info to find album ID
	manifest, err := ampapi.GetSongResp(storefront, songId, r.Config.General.Language, token)
	if err != nil {
		logx.ErrorT("app.rip.song_response_failed", err)
		return err
	}

	songData := manifest.Data[0]
	albumId := songData.Relationships.Albums.Data[0].ID

	// Use album approach but only download the specific song
	err = r.ripAlbum(albumId, token, storefront, mediaUserToken, songId)
	if err != nil {
		logx.ErrorT("app.rip.song_rip_failed", err)
		return err
	}

	return nil
}
