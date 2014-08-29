deploy:
	go build
	scp playadm 10.226.150.158:/usr/local/bin
	scp playadm 10.226.150.136:/usr/local/bin
	rm playadm
