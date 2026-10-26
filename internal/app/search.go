package app

import (
	"amdl/internal/amp-api"
	"amdl/internal/i18n"
	"amdl/internal/logx"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"strings"
)

type SearchResultItem struct {
	Type   string
	Name   string
	Detail string
	URL    string
	ID     string
}

// QualityOption holds information about a downloadable quality.

type QualityOption struct {
	ID          string
	Description string
}

// setDlFlags configures the global download flags based on the user's quality selection.

func (r *Runner) setDlFlags(quality string) {
	r.Flags.Atmos = false
	r.Flags.AAC = false

	switch quality {
	case "atmos":
		r.Flags.Atmos = true
		logx.OKT("app.flow.quality_atmos")
	case "aac":
		r.Flags.AAC = true
		r.Config.Media.AacType = "aac"
		logx.OKT("app.flow.quality_aac")
	case "alac":
		logx.OKT("app.flow.quality_alac")
	}
}

// promptForQuality asks the user to select a download quality for the chosen media.

func (r *Runner) promptForQuality(item SearchResultItem, token string) (string, error) {
	if item.Type == "Artist" {
		logx.StepT("app.flow.artist_selected")
		return "default", nil
	}

	logx.StepT("app.flow.fetch_qualities", item.Name)

	qualities := []QualityOption{
		{ID: "alac", Description: i18n.T("app.flow.quality_desc_alac")},
		{ID: "aac", Description: i18n.T("app.flow.quality_desc_aac")},
		{ID: "atmos", Description: i18n.T("app.flow.quality_desc_atmos")},
	}
	qualityOptions := []string{}
	for _, q := range qualities {
		qualityOptions = append(qualityOptions, q.Description)
	}

	prompt := &survey.Select{
		Message:  i18n.T("app.flow.quality_prompt"),
		Options:  qualityOptions,
		PageSize: 5,
	}

	// survey 直接读 stdin，先发 ##prompt 让 App 弹选择器。
	logx.Print("##prompt\n")
	selectedIndex := 0
	err := survey.AskOne(prompt, &selectedIndex)
	if err != nil {
		// This can happen if the user presses Ctrl+C
		return "", nil
	}

	return qualities[selectedIndex].ID, nil
}

// handleSearch manages the entire interactive search process.

func (r *Runner) handleSearch(searchType string, queryParts []string, token string) (string, error) {
	query := strings.Join(queryParts, " ")
	validTypes := map[string]bool{"album": true, "song": true, "artist": true}
	if !validTypes[searchType] {
		return "", fmt.Errorf("invalid search type: %s. Use 'album', 'song', or 'artist'", searchType)
	}

	typeKey := "app.flow.kind_albums"
	switch searchType {
	case "song":
		typeKey = "app.flow.kind_songs"
	case "artist":
		typeKey = "app.flow.kind_artists"
	}
	logx.StepT("app.flow.search_start", i18n.T(typeKey), query, r.Config.General.Storefront)

	offset := 0
	limit := 15 // Increased limit for better navigation

	apiSearchType := searchType + "s"

	for {
		searchResp, err := ampapi.Search(r.Config.General.Storefront, query, apiSearchType, r.Config.General.Language, token, limit, offset)
		if err != nil {
			return "", fmt.Errorf("error fetching search results: %w", err)
		}

		var items []SearchResultItem
		var displayOptions []string
		hasNext := false

		// Special options for navigation
		prevPageOpt := i18n.T("app.flow.page_prev")
		nextPageOpt := i18n.T("app.flow.page_next")

		// Add previous page option if applicable
		if offset > 0 {
			displayOptions = append(displayOptions, prevPageOpt)
		}

		switch searchType {
		case "album":
			if searchResp.Results.Albums != nil {
				for _, item := range searchResp.Results.Albums.Data {
					year := ""
					if len(item.Attributes.ReleaseDate) >= 4 {
						year = item.Attributes.ReleaseDate[:4]
					}
					trackInfo := i18n.T("app.flow.track_count", item.Attributes.TrackCount)
					displayOptions = append(displayOptions, i18n.T("app.flow.search_album_line", item.Attributes.Name, item.Attributes.ArtistName, year, trackInfo))
					items = append(items, SearchResultItem{Type: "Album", URL: item.Attributes.URL, ID: item.ID})
				}
				hasNext = searchResp.Results.Albums.Next != ""
			}
		case "song":
			if searchResp.Results.Songs != nil {
				for _, item := range searchResp.Results.Songs.Data {
					displayOptions = append(displayOptions, i18n.T("app.flow.search_song_line", item.Attributes.Name, item.Attributes.ArtistName, item.Attributes.AlbumName))
					items = append(items, SearchResultItem{Type: "Song", URL: item.Attributes.URL, ID: item.ID})
				}
				hasNext = searchResp.Results.Songs.Next != ""
			}
		case "artist":
			if searchResp.Results.Artists != nil {
				for _, item := range searchResp.Results.Artists.Data {
					detail := ""
					if len(item.Attributes.GenreNames) > 0 {
						detail = strings.Join(item.Attributes.GenreNames, ", ")
					}
					displayOptions = append(displayOptions, i18n.T("app.flow.search_artist_line", item.Attributes.Name, detail))
					items = append(items, SearchResultItem{Type: "Artist", URL: item.Attributes.URL, ID: item.ID})
				}
				hasNext = searchResp.Results.Artists.Next != ""
			}
		}

		if len(items) == 0 && offset == 0 {
			logx.WarnT("app.flow.no_results")
			return "", nil
		}

		// Add next page option if applicable
		if hasNext {
			displayOptions = append(displayOptions, nextPageOpt)
		}

		prompt := &survey.Select{
			Message:  i18n.T("app.flow.navigate_prompt"),
			Options:  displayOptions,
			PageSize: limit, // Show a full page of results
		}

		// survey 直接读 stdin，先发 ##prompt 让 App 弹选择器。
		logx.Print("##prompt\n")
		selectedIndex := 0
		err = survey.AskOne(prompt, &selectedIndex)
		if err != nil {
			// User pressed Ctrl+C
			return "", nil
		}

		selectedOption := displayOptions[selectedIndex]

		// Handle pagination
		if selectedOption == nextPageOpt {
			offset += limit
			continue
		}
		if selectedOption == prevPageOpt {
			offset -= limit
			continue
		}

		// Adjust index to match the `items` slice if "Previous Page" was an option
		itemIndex := selectedIndex
		if offset > 0 {
			itemIndex--
		}

		selectedItem := items[itemIndex]

		quality, err := r.promptForQuality(selectedItem, token)
		if err != nil {
			return "", fmt.Errorf("could not process quality selection: %w", err)
		}
		if quality == "" { // User cancelled quality selection
			logx.WarnT("app.flow.selection_cancelled")
			return "", nil
		}

		if quality != "default" {
			r.setDlFlags(quality)
		}

		return selectedItem.URL, nil
	}
}

// END: New functions for search functionality

// CONVERSION FEATURE: Determine if source codec is lossy (rough heuristic by extension/codec name).
