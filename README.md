# Postfix Exporter for Prometheus

This is a simple server that periodically scrapes postfix queue status and exports them via HTTP for Prometheus
consumption.

To install it:

```bash
sudo apt-get install mercurial
git clone https://github.com/dhasthagheer/postfix_exporter.git
cd postfix_exporter
make
```

To run it:

```bash
sudo ./postfix_exporter [flags]
```

Help on flags:
```bash
./postfix_exporter --help
```
