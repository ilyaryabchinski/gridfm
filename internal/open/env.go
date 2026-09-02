package open

import "os"

func execEnv(key string) string { return os.Getenv(key) }
