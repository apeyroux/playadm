deploy:
	go build
	scp playadm 10.226.150.158:/usr/local/bin
	scp playadm 10.226.150.136:/usr/local/bin
	scp -r tpl 10.226.150.158:
	scp -r static 10.226.150.158:
	rm playadm
