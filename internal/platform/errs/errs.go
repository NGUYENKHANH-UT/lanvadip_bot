package errs

import (
	"lanvadip-bot/internal/platform/transport"
	"net/http"

	"go.uber.org/zap"
)

func InternalServerError(w http.ResponseWriter, r *http.Request, err error, logger *zap.SugaredLogger) {
	logger.Error("Internal server error", zap.Error(err), zap.String("method", r.Method), zap.String("url", r.URL.String()))

	transport.WriteJSONError(w, http.StatusInternalServerError, "the server encountered a problem")
}
