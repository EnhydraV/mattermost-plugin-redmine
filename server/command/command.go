package command

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/EnhydraV/mattermost-plugin-redmine/server/store/kvstore"
)

const Trigger = "redmine"

// Identifiant de projet Redmine : minuscules, chiffres, tirets et soulignés, pas uniquement numérique.
var projectIdentifierPattern = regexp.MustCompile(`^[a-z0-9_-]{1,100}$`)
var onlyDigits = regexp.MustCompile(`^[0-9]+$`)

// Deps isole ce dont la commande a besoin, pour rester testable sans serveur.
type Deps struct {
	Store kvstore.ChannelProjects

	// CanManageChannel dit si l'utilisateur peut modifier le lien du canal (admin de canal).
	CanManageChannel func(userID, channelID string) bool

	// DefaultProject est le projet de repli configuré, affiché par « status ».
	DefaultProject func() string
}

type Handler struct {
	deps Deps
}

func Register(client *pluginapi.Client) error {
	link := model.NewAutocompleteData("link", "[identifiant-projet]", "Lier ce canal à un projet Redmine (admins de canal)")
	link.AddTextArgument("Identifiant du projet Redmine (celui de l'URL, ex. mon-projet)", "[identifiant-projet]", "")
	unlink := model.NewAutocompleteData("unlink", "", "Retirer le lien de ce canal vers un projet Redmine")
	status := model.NewAutocompleteData("status", "", "Afficher le projet Redmine lié à ce canal")

	root := model.NewAutocompleteData(Trigger, "[link|unlink|status]", "Lier ce canal à un projet Redmine")
	root.AddCommand(link)
	root.AddCommand(unlink)
	root.AddCommand(status)

	return client.SlashCommand.Register(&model.Command{
		Trigger:          Trigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Lier ce canal à un projet Redmine",
		AutoCompleteHint: "[link|unlink|status]",
		AutocompleteData: root,
	})
}

func NewHandler(deps Deps) *Handler {
	return &Handler{deps: deps}
}

func ephemeral(text string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}
}

func ValidProjectIdentifier(identifier string) bool {
	return projectIdentifierPattern.MatchString(identifier) && !onlyDigits.MatchString(identifier)
}

func (h *Handler) Handle(args *model.CommandArgs) *model.CommandResponse {
	fields := strings.Fields(args.Command)
	if len(fields) < 2 || strings.TrimPrefix(fields[0], "/") != Trigger {
		return ephemeral(usage())
	}

	switch fields[1] {
	case "link":
		if len(fields) < 3 {
			return ephemeral("Il manque l'identifiant du projet : `/redmine link <identifiant-projet>`.")
		}
		return h.link(args, fields[2])
	case "unlink":
		return h.unlink(args)
	case "status":
		return h.status(args)
	default:
		return ephemeral(usage())
	}
}

func usage() string {
	return "Commandes disponibles :\n" +
		"- `/redmine link <identifiant-projet>` : lier ce canal à un projet Redmine (admins de canal)\n" +
		"- `/redmine unlink` : retirer le lien\n" +
		"- `/redmine status` : afficher le projet lié"
}

func (h *Handler) link(args *model.CommandArgs, identifier string) *model.CommandResponse {
	if !h.deps.CanManageChannel(args.UserId, args.ChannelId) {
		return ephemeral("Seuls les administrateurs du canal peuvent lier un projet Redmine.")
	}
	if !ValidProjectIdentifier(identifier) {
		return ephemeral(fmt.Sprintf("`%s` n'est pas un identifiant de projet Redmine valide (minuscules, chiffres, `-` et `_`, pas uniquement des chiffres). C'est celui qui figure dans l'URL du projet.", identifier))
	}
	if err := h.deps.Store.SetProject(args.ChannelId, identifier); err != nil {
		return ephemeral("Impossible d'enregistrer le lien : " + err.Error())
	}
	return ephemeral(fmt.Sprintf("Ce canal est maintenant lié au projet Redmine `%s`.", identifier))
}

func (h *Handler) unlink(args *model.CommandArgs) *model.CommandResponse {
	if !h.deps.CanManageChannel(args.UserId, args.ChannelId) {
		return ephemeral("Seuls les administrateurs du canal peuvent retirer le lien vers un projet Redmine.")
	}
	if err := h.deps.Store.DeleteProject(args.ChannelId); err != nil {
		return ephemeral("Impossible de retirer le lien : " + err.Error())
	}
	return ephemeral("Ce canal n'est plus lié à un projet Redmine.")
}

func (h *Handler) status(args *model.CommandArgs) *model.CommandResponse {
	project, err := h.deps.Store.GetProject(args.ChannelId)
	if err != nil {
		return ephemeral("Impossible de lire le lien : " + err.Error())
	}
	if project != "" {
		return ephemeral(fmt.Sprintf("Ce canal est lié au projet Redmine `%s`.", project))
	}
	if fallback := h.deps.DefaultProject(); fallback != "" {
		return ephemeral(fmt.Sprintf("Ce canal n'est lié à aucun projet ; le projet par défaut `%s` sera utilisé.", fallback))
	}
	return ephemeral("Ce canal n'est lié à aucun projet et aucun projet par défaut n'est configuré : Redmine demandera de choisir le projet.")
}
