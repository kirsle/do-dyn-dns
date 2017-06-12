# Dynamic DNS on Digital Ocean

This is a Go program that uses [Digital Ocean](https://www.digitalocean.com/) as
a dynamic DNS provider. For example, you can point a domain name that resolves
to your home IP address, and if your home IP is dynamic and subject to change,
this program can update your DNS settings in Digital Ocean automatically.

After initial setup, you can run this program from a cron script. It will check
its public IPv4 and IPv6 addresses (using `ipv[46].myexternalip.com/raw`) and
compare them to the values it got the last time. If they have changed, it will
connect to the Digital Ocean API and update the DNS records.

For the DNS update, it will delete all `A` and `AAAA` records on the domain and
create new ones that point to the respective IP addresses. The domain itself
(`@`) and all subdomains (`*`) will have their records updated.

This program is free software and comes with no warranty. I wrote it to scratch
my own itch. Your mileage may vary.

## Setup

The first time you run the program it will ask you some interactive setup
questions. Here you will paste a Digital Ocean API access token, enter your
DNS domain name (you must have this already set up in DO's DNS control panel),
and whether you want to support IPv4 and IPv6.

You can re-run the setup again by running `do-dyn-dns -config`. The settings
are stored in a JSON file at `~/.config/do-dyn-dns.json` if you want to edit it
by hand.

## Example Run

```
% do-dyn-dns
do-dyn-dns v1.0.0

I'm going to ask a few questions to configure this app. (To reconfigure
it in the future, run `do-dyn-dns -config`

You'll need to log in to your Digital Ocean control panel and
create a Personal Access Token from the API dashboard, and paste
the token at the prompt below.

Digital Ocean Access Token: ****************************************

Next, you'll need to make sure your domain name is set up in the
DNS network settings in the Digital Ocean dashboard. At the prompt
below, enter the domain name as it appears in the dashboard,
for example: example.com

Domain name from your DNS dashboard: example.com
Support IPv4 (A records)? (Yes or No) y
Support IPv6 (AAAA records)? (Yes or No) n
DNS Record TTL? [1800] 1800

Found my IPv4 address: 93.184.216.34
My IP address has changed from when I last checked!
Updating DO DNS now!
Delete DNS record A: @ 93.184.216.34
Delete DNS record A: * 93.184.216.34
Creating A record: @ 93.184.216.34
Creating A record: * 93.184.216.34
```

## Command Line Options

* `-config`

  Runs through the first-time setup steps again.

* `-domain`

  You can temporarily override which domain name to use. This will cause the
  DNS updates to be applied to this domain instead of the one saved in the
  config file from your first run.

* `-force`

  Force the program to update Digital Ocean DNS records, even if the IP
  addresses it found haven't changed from the ones it saw previously.

## License

```
The MIT License (MIT)

Copyright (c) 2017 Noah Petherbridge

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
