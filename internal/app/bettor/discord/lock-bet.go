package discord

import (
	"context"
	"fmt"

	"github.com/bufbuild/connect-go"
	"github.com/bwmarrin/discordgo"
	api "github.com/elh/bettor/api/bettor/v1alpha"
)

var lockBetCommand = &discordgo.ApplicationCommand{
	Name:        "lock-bet",
	Description: "Lock a bet preventing further bets. Only the bet creator can lock the bet",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "bet",
			Description:  "Bet",
			Required:     true,
			MinLength:    &one,
			MaxLength:    1024,
			Autocomplete: true,
		},
	},
}

// LockBet is the handler for the /lock-bet command.
func LockBet(ctx context.Context, client bettorClient) Handler {
	return func(s *discordgo.Session, event *discordgo.InteractionCreate) (*discordgo.InteractionResponseData, error) {
		_, _, options, err := commandArgs(event)
		if err != nil {
			return nil, CErr("Failed to handle command", err)
		}

		switch event.Type { //nolint:exhaustive
		case discordgo.InteractionApplicationCommand:
			resp, err := client.LockMarket(ctx, &connect.Request[api.LockMarketRequest]{Msg: &api.LockMarketRequest{Name: options["bet"].StringValue()}})
			if err != nil {
				return nil, CErr("Failed to lock bet", err)
			}
			market := resp.Msg.GetMarket()

			userResp, err := client.GetUser(ctx, &connect.Request[api.GetUserRequest]{Msg: &api.GetUserRequest{Name: market.GetCreator()}})
			if err != nil {
				return nil, CErr("Failed to lookup bet creator", err)
			}
			marketCreator := userResp.Msg.GetUser()

			bets, bettors, err := getMarketBets(ctx, client, market.GetName())
			if err != nil {
				return nil, CErr("Failed to lookup bets", err)
			}

			msgformat, margs := formatMarket(market, marketCreator, bets, bettors)
			msgformat = "🎲 🔒 No more bets! `/settle-bet` when there is a winner.\n\n" + msgformat
			return &discordgo.InteractionResponseData{Content: localized.Sprintf(msgformat, margs...)}, nil
		case discordgo.InteractionApplicationCommandAutocomplete:
			guildID, discordUserID, _, err := commandArgs(event)
			if err != nil {
				return nil, CErr("Failed to handle command", err)
			}
			bettorUser, err := getUserOrCreateIfNotExist(ctx, client, guildID, discordUserID)
			if err != nil {
				return nil, CErr("Failed to lookup (or create nonexistent) user", err)
			}

			var choices []*discordgo.ApplicationCommandOptionChoice
			resp, err := client.ListMarkets(ctx, &connect.Request[api.ListMarketsRequest]{Msg: &api.ListMarketsRequest{
				Book:     guildBookName(guildID),
				Status:   api.Market_STATUS_OPEN,
				PageSize: 25,
			}})
			if err != nil {
				return nil, CErr("Failed to lookup bets", err)
			}
			for _, market := range resp.Msg.GetMarkets() {
				if market.GetCreator() != bettorUser.GetName() {
					continue
				}
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  market.GetTitle(),
					Value: market.GetName(),
				})
			}
			return &discordgo.InteractionResponseData{Choices: withDefaultChoices(choices)}, nil
		default:
			return nil, CErr("Something went wrong", fmt.Errorf("unexpected event type %v", event.Type))
		}
	}
}
