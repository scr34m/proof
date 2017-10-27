Sentry configuration
===

```
$client = new Raven_Client('http://any:data@10.33.214.1:2017/track/1');
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
