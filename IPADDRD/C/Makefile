all: ipaddrd

# https://stackoverflow.com/questions/20051202/how-does-gcc-ld-find-zlib-so >> "-lz"
ipaddrd: inet.c inet.h
	$(CC) $< -o $@ -pthread -lz -luuid
#-O0
