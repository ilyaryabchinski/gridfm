//go:build darwin && cgo

package operations

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
char *gridfmTrash(const char *path, char **destination);
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

func trashItem(ctx context.Context, src string, _ conflictResolver, _ func(done, total int64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := trashNative(src)
	return err
}

func trashNative(src string) (string, error) {
	path := C.CString(src)
	defer C.free(unsafe.Pointer(path))
	var destination *C.char
	result := C.gridfmTrash(path, &destination)
	if result != nil {
		defer C.free(unsafe.Pointer(result))
		return "", fmt.Errorf("trash %q: %s", src, C.GoString(result))
	}
	defer C.free(unsafe.Pointer(destination))

	return C.GoString(destination), nil
}
