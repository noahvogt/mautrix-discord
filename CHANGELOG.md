# v0.6.4 (2023-11-16)

* Changed error messages to be sent in a thread if the errored message was in
  a thread.

# v0.6.3 (2023-10-16)

* Fixed op7 reconnects during connection causing the bridge to get stuck
  disconnected.
* Fixed double puppet of recipient joining DM portals when both ends of a DM
  are using the same bridge.

# v0.6.2 (2023-09-16)

* Added support for double puppeting with arbitrary `as_token`s.
  See [docs](https://docs.mau.fi/bridges/general/double-puppeting.html#appservice-method-new) for more info.
* Adjusted markdown parsing rules to allow inline links in normal messages.
* Fixed panic if redacting an attachment fails.
* Fixed panic when handling video embeds with no URLs
  (thanks to [@odrling] in [#110]).

[@odrling]: https://github.com/odrling
[#110]: https://github.com/mautrix/discord/pull/110

# v0.6.1 (2023-08-16)

* Bumped minimum Go version to 1.20.
* Fixed all logged-in users being invited to existing portal rooms even if they
  don't have permission to view the channel on Discord.
* Fixed gif links not being treated as embeds if the canonical URL is different
  than the URL in the message body.

# v0.6.0 (2023-07-16)

* Added initial support for backfilling threads.
* Exposed `Application` flag to displayname template.
* Changed `m.emote` bridging to use italics on Discord.
* Updated Docker image to Alpine 3.18.
* Added limit to parallel media transfers to avoid high memory usage if lots
  of messages are received at the same time.
* Fixed guilds being unbridged if Discord has server issues and temporarily
  marks a guild as unavailable.
* Fixed using `guilds bridge` command without `--entire` flag.
* Fixed panic if lottieconverter isn't installed.
* Fixed relay webhook secret being leaked in network error messages.

# v0.5.0 (2023-06-16)

* Added support for intentional mentions in Matrix (MSC3952).
* Added `GlobalName` variable to displayname templates and updated the default
  template to prefer it over usernames.
* Added `Webhook` variable to displayname templates to allow determining if a
  ghost user is a webhook.
* Added guild profiles and webhook profiles as a custom field in Matrix
  message events.
* Added support for bulk message delete from Discord.
* Added support for appservice websockets.
* Enabled parsing headers (`#`) in Discord markdown.
* Messages that consist of a single image link are now bridged as images to
  closer match Discord.
* Stopped bridging incoming typing notifications from users who are logged into
  the bridge to prevent echoes.

# v0.4.0 (2023-05-16)

* Added bridging of friend nicks into DM room names.
* Added option to bypass homeserver for Discord media.
  See [docs](https://docs.mau.fi/bridges/go/discord/direct-media.html) for more info.
* Added conversion of replies to embeds when sending messages via webhook.
* Added option to disable caching reuploaded media. This may be necessary when
  using a media repo that doesn't create a unique mxc URI for each upload.
* Added option to disable uploading files directly to the Discord CDN
  (and send as form parts in the message send request instead).
* Improved formatting of error messages returned by Discord.
* Enabled discordgo info logs by default.
* Fixed limited backfill always stopping after 50 messages
  (thanks to [@odrling] in [#81]).
* Fixed startup sync to sync most recent private channels first.
* Fixed syncing group DM participants when they change.
* Fixed bridging animated emojis in messages.
* Stopped handling all message edits from relay webhook to prevent incorrect
  edits.
* Possibly fixed inviting to portal rooms when multiple Matrix users use the
  bridge.

[@odrling]: https://github.com/odrling
[#81]: https://github.com/mautrix/discord/pull/81

# v0.3.0 (2023-04-16)

* Added support for backfilling on room creation and missed messages on startup.
* Added options to automatically ratchet/delete megolm sessions to minimize
  access to old messages.
* Added basic support for incoming voice messages.

# v0.2.0 (2023-03-16)

* Switched to zerolog for logging.
  * The basic log config will be migrated automatically, but you may want to
    tweak it as the options are different.
* Added support for logging in with a bot account.
  The [Authentication docs](https://docs.mau.fi/bridges/go/discord/authentication.html)
  have been updated with instructions for creating a bot.
* Added support for relaying messages for unauthenticated users using a webhook.
  See [docs](https://docs.mau.fi/bridges/go/discord/relay.html) for instructions.
* Added commands to bridge and unbridge channels manually.
* Added `ping` command.
* Added support for gif stickers from Discord.
* Changed mention bridging so mentions for users logged into the bridge use the
  Matrix user's MXID even if double puppeting is not enabled.
* Actually fixed ghost user info not being synced when receiving reactions.
* Fixed uncommon bug with sending messages that only occurred after login
  before restarting the bridge.
* Fixed guild name not being synced immediately after joining a new guild.
* Fixed variation selectors when bridging emojis to Discord.

# v0.1.1 (2023-02-16)

* Started automatically subscribing to bridged guilds. This fixes two problems:
  * Typing notifications should now work automatically in guilds.
  * Huge guilds now actually get messages bridged.
* Added support for converting animated lottie stickers to raster formats using
  [lottieconverter](https://github.com/sot-tech/LottieConverter).
* Added basic bridging for call start and guild join messages.
* Improved markdown parsing to disable more features that don't exist on Discord.
* Removed width from inline images (e.g. in the `guilds status` output) to
  handle non-square images properly.
* Fixed ghost user info not being synced when receiving reactions.

# v0.1.0 (2023-01-29)

Initial release.
