Minimal Sentry alternative / drop-in replacement for development / local use, easy install with out any extra dependencies.
===

Sentry configuration
===

```
$client = new Raven_Client('http://any:data@10.33.214.1:2017/track/1');
```

Auth mode configuration file
===

```
[[user]]
name = "Gabor Gyorvari"
email = "scr34m@gmail.com"
password = "1"
enabled = true

[[site]]
name = "1"
username = "a4f7646fd83544dd9499c18561338d56"
password = "62b2a152380044f28753c26a83cf2ee3"
enabled = true
```

Install as a macOS service
===

```
cp misc/proof.plist ~/Library/LaunchAgents
mkdir /usr/local/proof/
cp -r assets /usr/local/proof/
cp -r tpl /usr/local/proof/
cp proof /usr/local/proof/
launchctl load ~/Library/LaunchAgents/proof.plist
launchctl list
```

For macOS notifications the terminal-notifier is required:

```
brew install terminal-notifier
```