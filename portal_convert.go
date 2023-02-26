// mautrix-discord - A Matrix-Discord puppeting bridge.
// Copyright (C) 2023 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/appservice"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
)

type ConvertedMessage struct {
	AttachmentID string

	Type    event.Type
	Content *event.MessageEventContent
	Extra   map[string]any
}

func (portal *Portal) createMediaFailedMessage(bridgeErr error) *event.MessageEventContent {
	return &event.MessageEventContent{
		Body:    fmt.Sprintf("Failed to bridge media: %v", bridgeErr),
		MsgType: event.MsgNotice,
	}
}

const DiscordStickerSize = 160

func (portal *Portal) convertDiscordFile(typeName string, intent *appservice.IntentAPI, id, url string, content *event.MessageEventContent) *event.MessageEventContent {
	meta := AttachmentMeta{AttachmentID: id, MimeType: content.Info.MimeType}
	if typeName == "sticker" && content.Info.MimeType == "application/json" {
		meta.Converter = portal.bridge.convertLottie
	}
	dbFile, err := portal.bridge.copyAttachmentToMatrix(intent, url, portal.Encrypted, meta)
	if err != nil {
		portal.log.Errorfln("Error copying attachment %s to Matrix: %v", id, err)
		return portal.createMediaFailedMessage(err)
	}
	if typeName == "sticker" && content.Info.MimeType == "application/json" {
		content.Info.MimeType = dbFile.MimeType
	}
	content.Info.Size = dbFile.Size
	if content.Info.Width == 0 && content.Info.Height == 0 {
		content.Info.Width = dbFile.Width
		content.Info.Height = dbFile.Height
	}
	if content.Info.Width == 0 && content.Info.Height == 0 && typeName == "sticker" {
		content.Info.Width = DiscordStickerSize
		content.Info.Height = DiscordStickerSize
	}
	if dbFile.DecryptionInfo != nil {
		content.File = &event.EncryptedFileInfo{
			EncryptedFile: *dbFile.DecryptionInfo,
			URL:           dbFile.MXC.CUString(),
		}
	} else {
		content.URL = dbFile.MXC.CUString()
	}

	if typeName == "sticker" && (content.Info.Width > DiscordStickerSize || content.Info.Height > DiscordStickerSize) {
		if content.Info.Width > content.Info.Height {
			content.Info.Height /= content.Info.Width / DiscordStickerSize
			content.Info.Width = DiscordStickerSize
		} else if content.Info.Width < content.Info.Height {
			content.Info.Width /= content.Info.Height / DiscordStickerSize
			content.Info.Height = DiscordStickerSize
		} else {
			content.Info.Width = DiscordStickerSize
			content.Info.Height = DiscordStickerSize
		}
	}
	return content
}

func (portal *Portal) convertDiscordSticker(intent *appservice.IntentAPI, sticker *discordgo.Sticker) *ConvertedMessage {
	var mime string
	switch sticker.FormatType {
	case discordgo.StickerFormatTypePNG:
		mime = "image/png"
	case discordgo.StickerFormatTypeAPNG:
		mime = "image/apng"
	case discordgo.StickerFormatTypeLottie:
		mime = "application/json"
	case discordgo.StickerFormatTypeGIF:
		mime = "image/gif"
	default:
		portal.log.Warnfln("Unknown sticker format %d in %s", sticker.FormatType, sticker.ID)
	}
	return &ConvertedMessage{
		Type: event.EventSticker,
		Content: portal.convertDiscordFile("sticker", intent, sticker.ID, sticker.URL(), &event.MessageEventContent{
			Body: sticker.Name, // TODO find description from somewhere?
			Info: &event.FileInfo{
				MimeType: mime,
			},
		}),
	}
}

func (portal *Portal) convertDiscordAttachment(intent *appservice.IntentAPI, att *discordgo.MessageAttachment) *ConvertedMessage {
	content := &event.MessageEventContent{
		Body: att.Filename,
		Info: &event.FileInfo{
			Height:   att.Height,
			MimeType: att.ContentType,
			Width:    att.Width,

			// This gets overwritten later after the file is uploaded to the homeserver
			Size: att.Size,
		},
	}
	if att.Description != "" {
		content.Body = att.Description
		content.FileName = att.Filename
	}

	switch strings.ToLower(strings.Split(att.ContentType, "/")[0]) {
	case "audio":
		content.MsgType = event.MsgAudio
	case "image":
		content.MsgType = event.MsgImage
	case "video":
		content.MsgType = event.MsgVideo
	default:
		content.MsgType = event.MsgFile
	}
	content = portal.convertDiscordFile("attachment", intent, att.ID, att.URL, content)
	return &ConvertedMessage{
		Type:    event.EventMessage,
		Content: content,
	}
}

func (portal *Portal) convertDiscordVideoEmbed(intent *appservice.IntentAPI, embed *discordgo.MessageEmbed) *ConvertedMessage {
	attachmentID := fmt.Sprintf("video_%s", embed.URL)
	dbFile, err := portal.bridge.copyAttachmentToMatrix(intent, embed.Video.ProxyURL, portal.Encrypted, NoMeta)
	if err != nil {
		return &ConvertedMessage{
			AttachmentID: attachmentID,
			Type:         event.EventMessage,
			Content:      portal.createMediaFailedMessage(err),
		}
	}

	content := &event.MessageEventContent{
		MsgType: event.MsgVideo,
		Body:    embed.URL,
		Info: &event.FileInfo{
			Width:    embed.Video.Width,
			Height:   embed.Video.Height,
			MimeType: dbFile.MimeType,

			Size: dbFile.Size,
		},
	}
	if content.Info.Width == 0 && content.Info.Height == 0 {
		content.Info.Width = dbFile.Width
		content.Info.Height = dbFile.Height
	}
	if dbFile.DecryptionInfo != nil {
		content.File = &event.EncryptedFileInfo{
			EncryptedFile: *dbFile.DecryptionInfo,
			URL:           dbFile.MXC.CUString(),
		}
	} else {
		content.URL = dbFile.MXC.CUString()
	}
	extra := map[string]any{}
	if embed.Type == discordgo.EmbedTypeGifv {
		extra["info"] = map[string]any{
			"fi.mau.discord.gifv":  true,
			"fi.mau.loop":          true,
			"fi.mau.autoplay":      true,
			"fi.mau.hide_controls": true,
			"fi.mau.no_audio":      true,
		}
	}
	return &ConvertedMessage{
		AttachmentID: attachmentID,
		Type:         event.EventMessage,
		Content:      content,
		Extra:        extra,
	}
}

func (portal *Portal) convertDiscordMessage(intent *appservice.IntentAPI, msg *discordgo.Message) []*ConvertedMessage {
	predictedLength := len(msg.Attachments) + len(msg.StickerItems)
	if msg.Content != "" {
		predictedLength++
	}
	parts := make([]*ConvertedMessage, 0, predictedLength)
	if textPart := portal.convertDiscordTextMessage(intent, msg); textPart != nil {
		parts = append(parts, textPart)
	}
	for _, att := range msg.Attachments {
		if part := portal.convertDiscordAttachment(intent, att); part != nil {
			parts = append(parts, part)
		}
	}
	for _, sticker := range msg.StickerItems {
		if part := portal.convertDiscordSticker(intent, sticker); part != nil {
			parts = append(parts, part)
		}
	}
	handledURLs := make(map[string]struct{})
	for _, embed := range msg.Embeds {
		// Ignore non-video embeds, they're handled in convertDiscordTextMessage
		if getEmbedType(embed) != EmbedVideo {
			continue
		}
		// Discord deduplicates embeds by URL. It makes things easier for us too.
		if _, handled := handledURLs[embed.URL]; handled {
			continue
		}
		handledURLs[embed.URL] = struct{}{}
		part := portal.convertDiscordVideoEmbed(intent, embed)
		if part != nil {
			parts = append(parts, part)
		}
	}
	return parts
}

const (
	embedHTMLWrapper         = `<blockquote class="discord-embed">%s</blockquote>`
	embedHTMLWrapperColor    = `<blockquote class="discord-embed" background-color="#%06X">%s</blockquote>`
	embedHTMLAuthorWithImage = `<p class="discord-embed-author"><img data-mx-emoticon height="24" src="%s" title="Author icon" alt="">&nbsp;<span>%s</span></p>`
	embedHTMLAuthorPlain     = `<p class="discord-embed-author"><span>%s</span></p>`
	embedHTMLAuthorLink      = `<a href="%s">%s</a>`
	embedHTMLTitleWithLink   = `<p class="discord-embed-title"><a href="%s"><strong>%s</strong></a></p>`
	embedHTMLTitlePlain      = `<p class="discord-embed-title"><strong>%s</strong></p>`
	embedHTMLDescription     = `<p class="discord-embed-description">%s</p>`
	embedHTMLFieldName       = `<th>%s</th>`
	embedHTMLFieldValue      = `<td>%s</td>`
	embedHTMLFields          = `<table class="discord-embed-fields"><tr>%s</tr><tr>%s</tr></table>`
	embedHTMLLinearField     = `<p class="discord-embed-field" x-inline="%s"><strong>%s</strong><br><span>%s</span></p>`
	embedHTMLImage           = `<p class="discord-embed-image"><img src="%s" alt="" title="Embed image"></p>`
	embedHTMLFooterWithImage = `<p class="discord-embed-footer"><sub><img data-mx-emoticon height="20" src="%s" title="Footer icon" alt="">&nbsp;<span>%s</span>%s</sub></p>`
	embedHTMLFooterPlain     = `<p class="discord-embed-footer"><sub><span>%s</span>%s</sub></p>`
	embedHTMLFooterOnlyDate  = `<p class="discord-embed-footer"><sub>%s</sub></p>`
	embedHTMLDate            = `<time datetime="%s">%s</time>`
	embedFooterDateSeparator = ` • `
)

func (portal *Portal) convertDiscordRichEmbed(intent *appservice.IntentAPI, embed *discordgo.MessageEmbed, msgID string, index int) string {
	var htmlParts []string
	if embed.Author != nil {
		var authorHTML string
		authorNameHTML := html.EscapeString(embed.Author.Name)
		if embed.Author.URL != "" {
			authorNameHTML = fmt.Sprintf(embedHTMLAuthorLink, embed.Author.URL, authorNameHTML)
		}
		authorHTML = fmt.Sprintf(embedHTMLAuthorPlain, authorNameHTML)
		if embed.Author.ProxyIconURL != "" {
			dbFile, err := portal.bridge.copyAttachmentToMatrix(intent, embed.Author.ProxyIconURL, false, NoMeta)
			if err != nil {
				portal.log.Warnfln("Failed to reupload author icon in embed #%d of message %s: %v", index+1, msgID, err)
			} else {
				authorHTML = fmt.Sprintf(embedHTMLAuthorWithImage, dbFile.MXC, authorNameHTML)
			}
		}
		htmlParts = append(htmlParts, authorHTML)
	}
	if embed.Title != "" {
		var titleHTML string
		baseTitleHTML := portal.renderDiscordMarkdownOnlyHTML(embed.Title, false)
		if embed.URL != "" {
			titleHTML = fmt.Sprintf(embedHTMLTitleWithLink, html.EscapeString(embed.URL), baseTitleHTML)
		} else {
			titleHTML = fmt.Sprintf(embedHTMLTitlePlain, baseTitleHTML)
		}
		htmlParts = append(htmlParts, titleHTML)
	}
	if embed.Description != "" {
		htmlParts = append(htmlParts, fmt.Sprintf(embedHTMLDescription, portal.renderDiscordMarkdownOnlyHTML(embed.Description, true)))
	}
	for i := 0; i < len(embed.Fields); i++ {
		item := embed.Fields[i]
		if portal.bridge.Config.Bridge.EmbedFieldsAsTables {
			splitItems := []*discordgo.MessageEmbedField{item}
			if item.Inline && len(embed.Fields) > i+1 && embed.Fields[i+1].Inline {
				splitItems = append(splitItems, embed.Fields[i+1])
				i++
				if len(embed.Fields) > i+1 && embed.Fields[i+1].Inline {
					splitItems = append(splitItems, embed.Fields[i+1])
					i++
				}
			}
			headerParts := make([]string, len(splitItems))
			contentParts := make([]string, len(splitItems))
			for j, splitItem := range splitItems {
				headerParts[j] = fmt.Sprintf(embedHTMLFieldName, portal.renderDiscordMarkdownOnlyHTML(splitItem.Name, false))
				contentParts[j] = fmt.Sprintf(embedHTMLFieldValue, portal.renderDiscordMarkdownOnlyHTML(splitItem.Value, true))
			}
			htmlParts = append(htmlParts, fmt.Sprintf(embedHTMLFields, strings.Join(headerParts, ""), strings.Join(contentParts, "")))
		} else {
			htmlParts = append(htmlParts, fmt.Sprintf(embedHTMLLinearField,
				strconv.FormatBool(item.Inline),
				portal.renderDiscordMarkdownOnlyHTML(item.Name, false),
				portal.renderDiscordMarkdownOnlyHTML(item.Value, true),
			))
		}
	}
	if embed.Image != nil {
		dbFile, err := portal.bridge.copyAttachmentToMatrix(intent, embed.Image.ProxyURL, false, NoMeta)
		if err != nil {
			portal.log.Warnfln("Failed to reupload image in embed #%d of message %s: %v", index+1, msgID, err)
		} else {
			htmlParts = append(htmlParts, fmt.Sprintf(embedHTMLImage, dbFile.MXC))
		}
	}
	var embedDateHTML string
	if embed.Timestamp != "" {
		formattedTime := embed.Timestamp
		parsedTS, err := time.Parse(time.RFC3339, embed.Timestamp)
		if err != nil {
			portal.log.Warnfln("Failed to parse timestamp in embed #%d of message %s: %v", index+1, msgID, err)
		} else {
			formattedTime = parsedTS.Format(discordTimestampStyle('F').Format())
		}
		embedDateHTML = fmt.Sprintf(embedHTMLDate, embed.Timestamp, formattedTime)
	}
	if embed.Footer != nil {
		var footerHTML string
		var datePart string
		if embedDateHTML != "" {
			datePart = embedFooterDateSeparator + embedDateHTML
		}
		footerHTML = fmt.Sprintf(embedHTMLFooterPlain, html.EscapeString(embed.Footer.Text), datePart)
		if embed.Footer.ProxyIconURL != "" {
			dbFile, err := portal.bridge.copyAttachmentToMatrix(intent, embed.Footer.ProxyIconURL, false, NoMeta)
			if err != nil {
				portal.log.Warnfln("Failed to reupload footer icon in embed #%d of message %s: %v", index+1, msgID, err)
			} else {
				footerHTML = fmt.Sprintf(embedHTMLFooterWithImage, dbFile.MXC, html.EscapeString(embed.Footer.Text), datePart)
			}
		}
		htmlParts = append(htmlParts, footerHTML)
	} else if embed.Timestamp != "" {
		htmlParts = append(htmlParts, fmt.Sprintf(embedHTMLFooterOnlyDate, embedDateHTML))
	}

	if len(htmlParts) == 0 {
		return ""
	}

	compiledHTML := strings.Join(htmlParts, "")
	if embed.Color != 0 {
		compiledHTML = fmt.Sprintf(embedHTMLWrapperColor, embed.Color, compiledHTML)
	} else {
		compiledHTML = fmt.Sprintf(embedHTMLWrapper, compiledHTML)
	}
	return compiledHTML
}

type BeeperLinkPreview struct {
	mautrix.RespPreviewURL
	MatchedURL      string                   `json:"matched_url"`
	ImageEncryption *event.EncryptedFileInfo `json:"beeper:image:encryption,omitempty"`
}

func (portal *Portal) convertDiscordLinkEmbedImage(intent *appservice.IntentAPI, url string, width, height int, preview *BeeperLinkPreview) {
	dbFile, err := portal.bridge.copyAttachmentToMatrix(intent, url, portal.Encrypted, NoMeta)
	if err != nil {
		portal.log.Warnfln("Failed to copy image in URL preview: %v", err)
	} else {
		if width != 0 || height != 0 {
			preview.ImageWidth = width
			preview.ImageHeight = height
		} else {
			preview.ImageWidth = dbFile.Width
			preview.ImageHeight = dbFile.Height
		}
		preview.ImageSize = dbFile.Size
		preview.ImageType = dbFile.MimeType
		if dbFile.Encrypted {
			preview.ImageEncryption = &event.EncryptedFileInfo{
				EncryptedFile: *dbFile.DecryptionInfo,
				URL:           dbFile.MXC.CUString(),
			}
		} else {
			preview.ImageURL = dbFile.MXC.CUString()
		}
	}
}

func (portal *Portal) convertDiscordLinkEmbedToBeeper(intent *appservice.IntentAPI, embed *discordgo.MessageEmbed) *BeeperLinkPreview {
	var preview BeeperLinkPreview
	preview.MatchedURL = embed.URL
	preview.Title = embed.Title
	preview.Description = embed.Description
	if embed.Image != nil {
		portal.convertDiscordLinkEmbedImage(intent, embed.Image.ProxyURL, embed.Image.Width, embed.Image.Height, &preview)
	} else if embed.Thumbnail != nil {
		portal.convertDiscordLinkEmbedImage(intent, embed.Thumbnail.ProxyURL, embed.Thumbnail.Width, embed.Thumbnail.Height, &preview)
	}
	return &preview
}

const msgInteractionTemplateHTML = `<blockquote>
<a href="https://matrix.to/#/%s">%s</a> used <font color="#3771bb">/%s</font>
</blockquote>`

const msgComponentTemplateHTML = `<p>This message contains interactive elements. Use the Discord app to interact with the message.</p>`

type BridgeEmbedType int

const (
	EmbedUnknown BridgeEmbedType = iota
	EmbedRich
	EmbedLinkPreview
	EmbedVideo
)

func isActuallyLinkPreview(embed *discordgo.MessageEmbed) bool {
	// Sending YouTube links creates a video embed, but we want to bridge it as a URL preview,
	// so this is a hacky way to detect those.
	return embed.Video != nil && embed.Video.ProxyURL == ""
}

func getEmbedType(embed *discordgo.MessageEmbed) BridgeEmbedType {
	switch embed.Type {
	case discordgo.EmbedTypeLink, discordgo.EmbedTypeArticle:
		return EmbedLinkPreview
	case discordgo.EmbedTypeVideo:
		if isActuallyLinkPreview(embed) {
			return EmbedLinkPreview
		}
		return EmbedVideo
	case discordgo.EmbedTypeGifv:
		return EmbedVideo
	case discordgo.EmbedTypeRich, discordgo.EmbedTypeImage:
		return EmbedRich
	default:
		return EmbedUnknown
	}
}

func isPlainGifMessage(msg *discordgo.Message) bool {
	return len(msg.Embeds) == 1 && msg.Embeds[0].Video != nil && msg.Embeds[0].URL == msg.Content && msg.Embeds[0].Type == discordgo.EmbedTypeGifv
}

func (portal *Portal) convertDiscordTextMessage(intent *appservice.IntentAPI, msg *discordgo.Message) *ConvertedMessage {
	if msg.Type == discordgo.MessageTypeCall {
		return &ConvertedMessage{Type: event.EventMessage, Content: &event.MessageEventContent{
			MsgType: event.MsgEmote,
			Body:    "started a call",
		}}
	} else if msg.Type == discordgo.MessageTypeGuildMemberJoin {
		return &ConvertedMessage{Type: event.EventMessage, Content: &event.MessageEventContent{
			MsgType: event.MsgEmote,
			Body:    "joined the server",
		}}
	}
	var htmlParts []string
	if msg.Interaction != nil {
		puppet := portal.bridge.GetPuppetByID(msg.Interaction.User.ID)
		puppet.UpdateInfo(nil, msg.Interaction.User)
		htmlParts = append(htmlParts, fmt.Sprintf(msgInteractionTemplateHTML, puppet.MXID, puppet.Name, msg.Interaction.Name))
	}
	if msg.Content != "" && !isPlainGifMessage(msg) {
		htmlParts = append(htmlParts, portal.renderDiscordMarkdownOnlyHTML(msg.Content, false))
	}
	previews := make([]*BeeperLinkPreview, 0)
	for i, embed := range msg.Embeds {
		switch getEmbedType(embed) {
		case EmbedRich:
			htmlParts = append(htmlParts, portal.convertDiscordRichEmbed(intent, embed, msg.ID, i))
		case EmbedLinkPreview:
			previews = append(previews, portal.convertDiscordLinkEmbedToBeeper(intent, embed))
		case EmbedVideo:
			// Ignore video embeds, they're handled as separate messages
		default:
			portal.log.Warnfln("Unknown type %s in embed #%d of message %s", embed.Type, i+1, msg.ID)
		}
	}

	if len(msg.Components) > 0 {
		htmlParts = append(htmlParts, msgComponentTemplateHTML)
	}

	if len(htmlParts) == 0 {
		return nil
	}

	fullHTML := strings.Join(htmlParts, "\n")
	if !msg.MentionEveryone {
		fullHTML = strings.ReplaceAll(fullHTML, "@room", "@\u2063ro\u2063om")
	}

	content := format.HTMLToContent(fullHTML)
	extraContent := map[string]any{
		"com.beeper.linkpreviews": previews,
	}

	return &ConvertedMessage{Type: event.EventMessage, Content: &content, Extra: extraContent}
}
