// Copyright (c) 2017 Daniel Oaks <daniel@danieloaks.net>
// released under the MIT license

package irc

import (
	"fmt"
	"strings"

	"github.com/goshuirc/irc-go/ircfmt"
	"github.com/goshuirc/irc-go/ircmsg"
	"github.com/oragono/oragono/irc/sno"
)

// csHandler handles the /CS and /CHANSERV commands
func csHandler(server *Server, client *Client, msg ircmsg.IrcMessage) bool {
	server.chanservReceivePrivmsg(client, strings.Join(msg.Params, " "))
	return false
}

func (server *Server) chanservReceiveNotice(client *Client, message string) {
	// do nothing
}

// ChanServNotice sends the client a notice from ChanServ.
func (client *Client) ChanServNotice(text string) {
	client.Send(nil, fmt.Sprintf("ChanServ!services@%s", client.server.name), "NOTICE", client.nick, text)
}

func (server *Server) chanservReceivePrivmsg(client *Client, message string) {
	var params []string
	for _, p := range strings.Split(message, " ") {
		if len(p) > 0 {
			params = append(params, p)
		}
	}
	if len(params) < 1 {
		client.ChanServNotice("You need to run a command")
		//TODO(dan): dump CS help here
		return
	}

	command := strings.ToLower(params[0])
	server.logger.Debug("chanserv", fmt.Sprintf("Client %s ran command %s", client.nick, command))

	if command == "register" {
		if len(params) < 2 {
			client.ChanServNotice("Syntax: REGISTER <channel>")
			return
		}

		if !server.channelRegistrationEnabled {
			client.ChanServNotice("Channel registration is not enabled")
			return
		}

		channelName := params[1]
		channelKey, err := CasefoldChannel(channelName)
		if err != nil {
			client.ChanServNotice("Channel name is not valid")
			return
		}

		channelInfo := server.channels.Get(channelKey)
		if channelInfo == nil || !channelInfo.ClientIsAtLeast(client, ChannelOperator) {
			client.ChanServNotice("You must be an oper on the channel to register it")
			return
		}

		if client.account == &NoAccount {
			client.ChanServNotice("You must be logged in to register a channel")
			return
		}

		// this provides the synchronization that allows exactly one registration of the channel:
		err = channelInfo.SetRegistered(client.AccountName())
		if err != nil {
			client.ChanServNotice(err.Error())
			return
		}

		// registration was successful: make the database reflect it
		go server.channelRegistry.StoreChannel(channelInfo, true)

		client.ChanServNotice(fmt.Sprintf("Channel %s successfully registered", channelName))

		server.logger.Info("chanserv", fmt.Sprintf("Client %s registered channel %s", client.nick, channelName))
		server.snomasks.Send(sno.LocalChannels, fmt.Sprintf(ircfmt.Unescape("Channel registered $c[grey][$r%s$c[grey]] by $c[grey][$r%s$c[grey]]"), channelName, client.nickMaskString))

		// give them founder privs
		change := channelInfo.applyModeMemberNoMutex(client, ChannelFounder, Add, client.NickCasefolded())
		if change != nil {
			//TODO(dan): we should change the name of String and make it return a slice here
			//TODO(dan): unify this code with code in modes.go
			args := append([]string{channelName}, strings.Split(change.String(), " ")...)
			for _, member := range channelInfo.Members() {
				member.Send(nil, fmt.Sprintf("ChanServ!services@%s", client.server.name), "MODE", args...)
			}
		}
	} else {
		client.ChanServNotice("Sorry, I don't know that command")
	}
}
