package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/megaease/easeprobe/probe"
	log "github.com/sirupsen/logrus"
)

// Refer to:
// - Documents: https://birdie0.github.io/discord-webhooks-guide/index.html
// - Using https://discohook.org/ to preview

// Thumbnail use thumbnail in the embed. You can set only url of the thumbnail.
// There is no way to set width/height of the picture.
type Thumbnail struct {
	URL string `json:"url"`
}

// Fields allows you to use multiple title + description blocks in embed.
// fields is an array of field objects. Each object includes three values:
type Fields struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// Footer allows you to add footer to embed. footer is an object which includes two values:
//  - text - sets name for author object. Markdown is disabled here!!!
//  - icon_url - sets icon for author object. Requires text value.
type Footer struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url"`
}

// Author is an object which includes three values:
// - name - sets name.
// - url - sets link. Requires name value. If used, transforms name into hyperlink.
// - icon_url - sets avatar. Requires name value.
type Author struct {
	Name    string `json:"name"`
	URL     string `json:"url`
	IconURL string `json:icon_url`
}

// Embed is custom embeds for message sent by webhook.
// embeds is an array of embeds and can contain up to 10 embeds in the same message.
type Embed struct {
	Author      Author    `json:"author"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Color       int       `json:"color"`
	Description string    `json:"description"`
	Timestamp   string    `json:"timestamp"` //"YYYY-MM-DDTHH:MM:SS.MSSZ"
	Thumbnail   Thumbnail `json:"thumbnail"`
	Fields      []Fields  `json:"fields"`
	Footer      Footer    `json:"footer"`
}

// Discord is the struct for all of the discrod json.
type Discord struct {
	Username  string  `json:"username"`
	AvatarURL string  `json:"avatar_url"`
	Content   string  `json:"content"`
	Embeds    []Embed `json:"embeds"`
}

// NotifyConfig is the slack notification configuration
type NotifyConfig struct {
	WebhookURL string `yaml:"webhook"`
	Dry        bool   `yaml:"dry"`
}

// Kind return the type of Notify
func (c NotifyConfig) Kind() string {
	return "discord"
}

// Config configures the log files
func (c NotifyConfig) Config() error {
	if c.Dry {
		log.Infof("Notification %s is running on Dry mode!", c.Kind())
	}
	return nil
}

// NewDiscord new a discord object from a result
func (c NotifyConfig) NewDiscord(result probe.Result) Discord {
	discord := Discord{
		Username:  "Easeprobe",
		AvatarURL: "https://megaease.cn/favicon.png",
		Content:   "",
		Embeds:    []Embed{},
	}

	// using https://www.spycolor.com/ to pick color
	color := 1091331 //"#10a703" - green
	if result.Status != probe.StatusUp {
		color = 10945283 // "#a70303" - red
	}

	rtt := result.RoundTripTime.Round(time.Millisecond)
	description := fmt.Sprintf("%s %s - ⏱ %s\n```%s```",
		result.Status.Emoji(), result.Endpoint, rtt, result.Message)

	discord.Embeds = append(discord.Embeds, Embed{
		Author:      Author{},
		Title:       result.Title(),
		URL:         "",
		Color:       color,
		Description: description,
		Timestamp:   result.StartTime.UTC().Format(time.RFC3339),
		Thumbnail:   Thumbnail{URL: "https://megaease.cn/favicon.png"},
		Fields:      []Fields{},
		Footer:      Footer{Text: "Probed at", IconURL: "https://megaease.cn/favicon.png"},
	})
	return discord
}

// Notify write the message into the slack
func (c NotifyConfig) Notify(result probe.Result) {
	if c.Dry {
		c.DryNotify(result)
		return
	}

	discord := c.NewDiscord(result)
	err := c.SendDiscordNotification(discord)
	if err != nil {
		log.Errorf("%v", err)
		json, err := json.Marshal(discord)
		if err != nil {
			log.Errorf("Notify[%s] - %v", c.Kind(), discord)
		} else {
			log.Errorf("Notify[%s] - %s", c.Kind(), string(json))
		}
		return
	}
	log.Infof("Sent the Discord notification for %s (%s)!", result.Name, result.Endpoint)
}

// NewEmbed new a embed object from a result
func (c NotifyConfig) NewEmbed(result probe.Result) Embed {

	message := "**Availability**\n>\t" + " **Up**:  `%s`  **Down** `%s`  -  **SLA**: `%.2f %%`" +
		"\n**Probe Times**\n>\t**Total** : `%d` ( %s )" +
		"\n**Lastest Probe**\n>\t%s | %s" +
		"\n>\t`%s ` "

	desc := fmt.Sprintf(message,
		result.Stat.UpTime.Round(time.Second), result.Stat.DownTime.Round(time.Second), result.SLA(),
		result.Stat.Total, probe.StatStatusText(result.Stat, probe.Makerdown),
		result.StartTime.UTC().Format(result.TimeFormat), result.Status.Emoji()+" "+result.Status.String(),
		result.Message)

	embed := Embed{
		Author:      Author{},
		Title:       fmt.Sprintf("%s - %s", result.Name, result.Endpoint),
		URL:         "",
		Color:       239, // #0000ef - blue
		Description: desc,
		Timestamp:   "",
		Thumbnail:   Thumbnail{},
		Fields:      []Fields{},
		Footer:      Footer{},
	}

	return embed
}

// NewEmbeds return a discord with multiple Embed
func (c NotifyConfig) NewEmbeds(probers []probe.Prober) Discord {
	discord := Discord{
		Username:  "Easeprobe",
		AvatarURL: "https://megaease.cn/favicon.png",
		Content:   "**Overall SLA Report**",
		Embeds:    []Embed{},
	}
	for _, p := range probers {
		discord.Embeds = append(discord.Embeds, c.NewEmbed(*p.Result()))
	}

	return discord
}

// NotifyStat write the all probe stat message to slack
func (c NotifyConfig) NotifyStat(probers []probe.Prober) {
	if c.Dry {
		c.DryNotifyStat(probers)
		return
	}
	discord := c.NewEmbeds(probers)
	err := c.SendDiscordNotification(discord)
	if err != nil {
		log.Errorf("%v", err)
		json, err := json.Marshal(discord)
		if err != nil {
			log.Errorf("Notify[%s] - %v", c.Kind(), discord)
		} else {
			log.Errorf("Notify[%s] - %s", c, c.Kind(), string(json))
		}
		return
	}
	log.Infoln("Sent the Statstics to Slack Successfully!")
}

// DryNotify just log the notification message
func (c NotifyConfig) DryNotify(result probe.Result) {
	discord := c.NewDiscord(result)
	json, err := json.Marshal(discord)
	if err != nil {
		log.Errorf("error : %v", err)
		return
	}
	log.Infof("[%s] Dry notify - %s", c.Kind(), string(json))
}

// DryNotifyStat just log the notification message
func (c NotifyConfig) DryNotifyStat(probers []probe.Prober) {
	discord := c.NewEmbeds(probers)
	json, err := json.Marshal(discord)
	if err != nil {
		log.Errorf("error : %v", err)
		return
	}
	log.Infof("[%s] Dry notify - %s", c.Kind(), string(json))
}

// SendDiscordNotification will post to an 'Incoming Webhook' url setup in Discrod Apps.
func (c NotifyConfig) SendDiscordNotification(discord Discord) error {
	json, err := json.Marshal(discord)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.WebhookURL, bytes.NewBuffer([]byte(json)))
	if err != nil {
		return err
	}
	req.Close = true
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if resp.StatusCode != 204 {
		return fmt.Errorf("Error response from Discord [%d] - [%s]", resp.StatusCode, buf.String())
	}
	return nil
}