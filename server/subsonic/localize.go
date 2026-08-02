package subsonic

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/server/subsonic/responses"
)

// isPortuguese returns true when the request's Accept-Language header prefers
// Portuguese (pt or pt-BR). Used to localize user-visible API error messages.
func isPortuguese(r *http.Request) bool {
	for _, header := range r.Header.Values("Accept-Language") {
		for _, part := range strings.Split(header, ",") {
			lang := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
			if strings.HasPrefix(lang, "pt") {
				return true
			}
		}
	}
	return false
}

// canonicalPTBR holds pt-BR translations for the canonical Subsonic error codes.
var canonicalPTBR = map[int32]string{
	responses.ErrorGeneric:            "Ocorreu um erro",
	responses.ErrorMissingParameter:   "Parâmetro obrigatório ausente",
	responses.ErrorClientTooOld:       "Versão do protocolo Subsonic incompatível. O cliente precisa ser atualizado",
	responses.ErrorServerTooOld:       "Versão do protocolo Subsonic incompatível. O servidor precisa ser atualizado",
	responses.ErrorAuthenticationFail: "Usuário ou senha incorretos",
	responses.ErrorAuthorizationFail:  "O usuário não está autorizado para esta operação",
	responses.ErrorTrialExpired:       "O período de avaliação do servidor Subsonic terminou",
	responses.ErrorDataNotFound:       "Os dados solicitados não foram encontrados",
}

// messagePTBR translates the most common handler error patterns to pt-BR.
// The keys are the English format patterns used by newError(...).
var messagePTBR = map[string]string{
	"Internal error":            "Erro interno",
	"Internal Error":            "Erro interno",
	"Internal Server Error: %s": "Erro interno do servidor: %s",
	"data not found":            "Dados não encontrados",
	"too many concurrent transcodes, please retry shortly": "Muitos transcodings simultâneos, tente novamente em instantes",
	"playlist not found":                             "Playlist não encontrada",
	"Directory not found":                            "Pasta não encontrada",
	"Artist not found":                               "Artista não encontrado",
	"Album not found":                                "Álbum não encontrado",
	"Song not found":                                 "Música não encontrada",
	"Library not found or empty":                     "Biblioteca não encontrada ou vazia",
	"Library %d not found or not accessible":         "Biblioteca %d não encontrada ou inacessível",
	"Library with ID %d not found":                   "Biblioteca com ID %d não encontrada",
	"downloads are disabled":                         "Os downloads estão desabilitados",
	"Required id parameter is missing":               "O parâmetro id é obrigatório",
	"Required parameter is missing":                  "Parâmetro obrigatório ausente",
	"missing parameter: 'id'":                        "Parâmetro ausente: 'id'",
	"missing required parameter: mediaId":            "Parâmetro obrigatório ausente: mediaId",
	"missing required parameter: mediaType":          "Parâmetro obrigatório ausente: mediaType",
	"Avatar image not found":                         "Imagem de avatar não encontrada",
	"Artwork not found":                              "Imagem da capa não encontrada",
	"Jukebox is disabled":                            "Jukebox desabilitado",
	"Jukebox is admin only":                          "Jukebox disponível apenas para administradores",
	"invalid JSON request body":                      "Corpo da requisição JSON inválido",
	"media file not found: %s":                       "Arquivo de mídia não encontrado: %s",
	"error retrieving media file":                    "Erro ao obter o arquivo de mídia",
	"failed to create transcode token":               "Falha ao criar token de transcodificação",
	"invalid callback parameter":                     "Parâmetro de callback inválido",
	"Wrong username or password":                     "Usuário ou senha incorretos",
	"User is not authorized for the given operation": "O usuário não está autorizado para esta operação",
	"The requested data was not found":               "Os dados solicitados não foram encontrados",
}

// localizedMessage returns the error message localized to pt-BR when the client
// prefers Portuguese, falling back to the original English message otherwise.
func (e subError) localizedMessage(r *http.Request) string {
	if !isPortuguese(r) {
		return e.Error()
	}
	if len(e.messages) == 0 {
		if msg, ok := canonicalPTBR[e.code]; ok {
			return msg
		}
		return e.Error()
	}
	pattern, _ := e.messages[0].(string)
	if translated, ok := messagePTBR[pattern]; ok {
		pattern = translated
	}
	return fmt.Sprintf(pattern, e.messages[1:]...)
}
