RUNTIME := ../../runtime/c
GEN     := generated
SRC     := main.c $(GEN)/zip.c $(GEN)/dos_datetime.c
BIN     := zip-list

CC     ?= cc
CFLAGS ?= -std=c99 -O2 -Wall
CFLAGS += -I$(RUNTIME) -I$(GEN)
LDLIBS +=

.PHONY: all gen clean

all: $(BIN)

$(BIN): $(SRC) $(GEN)/zip.h $(GEN)/dos_datetime.h $(RUNTIME)/zanbato.h
	$(CC) $(CFLAGS) -o $@ $(SRC) $(LDLIBS)

gen:
	cd ../.. && go run ./cmd/zanbato-gen-c -I internal/third_party/kaitai_struct_formats -out examples/zip-list/$(GEN) internal/third_party/kaitai_struct_formats/archive/zip.ksy

clean:
	rm -f $(BIN)
