//go:build darwin && cgo

package operations

/*
#cgo LDFLAGS: -framework Foundation
#include <copyfile.h>
#include <stdlib.h>

static int gridfmCopyMetadata(const char *src, const char *dst) {
    return copyfile(src, dst, NULL, COPYFILE_XATTR | COPYFILE_ACL | COPYFILE_NOFOLLOW_SRC | COPYFILE_NOFOLLOW_DST);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func copyMetadata(src, dst string) error {
	from := C.CString(src)
	to := C.CString(dst)
	defer C.free(unsafe.Pointer(from))
	defer C.free(unsafe.Pointer(to))
	if result, err := C.gridfmCopyMetadata(from, to); result != 0 {
		return fmt.Errorf("copy metadata %q to %q: %w", src, dst, err)
	}

	return nil
}
