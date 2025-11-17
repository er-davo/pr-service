setup: copy-env copy-config

copy-env:
	cp .env.example .env

copy-config:
	cp config.yaml.example config.yaml