//go:build darwin && cgo

#import <Foundation/Foundation.h>
#include <string.h>

char *gridfmTrash(const char *path, char **destination) {
    @autoreleasepool {
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        NSError *error = nil;
        NSURL *result = nil;
        BOOL moved = [[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:&result error:&error];
        if (moved) {
            *destination = strdup([[result path] fileSystemRepresentation]);
            return NULL;
        }
        const char *message = error == nil ? "native Trash failed" : [[error localizedDescription] UTF8String];
        return strdup(message);
    }
}
