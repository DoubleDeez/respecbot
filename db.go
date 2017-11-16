package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

type User struct {
	Username string `xorm:"varchar(50) not null unique"`
	Respec   int    `xorm:"default 0"`
	ID       string `xorm:"varchar(50) pk"`
}

type Message struct {
	ID      string    `xorm:"varchar(50) pk"`
	Content string    `xorm:"varchar(2000) not null"`
	UserID  string    `xorm:"not null"`
	Respec  int       `xorm:"default 0"`
	Time    time.Time `xorm:"not null"`
}

type Reaction struct {
	Content   string    `xorm:"varchar(50) pk"`
	MessageID string    `xorm:"varchar(50) pk"`
	UserID    string    `xorm:"varchar(50) pk"`
	Time      time.Time `xorm:"not null"`
	Removed   time.Time `xorm:"default null"`
}

type Respec struct {
	ID         uint64    `xorm:"pk autoincr"`
	GiverID    string    `xorm:"not null"`
	ReceiverID string    `xorm:"not null"`
	Time       time.Time `xorm:"not null"`
	Respec     int       `xorm:"default 0"`
}

type Mention struct {
	GiverID    string    `xorm:"varchar(50) pk"`
	ReceiverID string    `xorm:"varchar(50) pk"`
	MessageID  string    `xorm:"varchar(50) pk"`
	Time       time.Time `xorm:"not null"`
	Respec     int       `xorm:"default 0"`
}

type joinReactionMessage struct {
	Reaction `xorm:"extends"`
	Message  `xorm:"extends"`
}

var engine *xorm.Engine

func InitDB() {
	engine = &xorm.Engine{}

	e, err := xorm.NewEngine("mysql", dbUser+":"+dbPassword+"@/"+dbName+"?charset=utf8mb4")
	if err != nil {
		panic(err)
	}

	engine = e

	engine.SetMapper(core.SameMapper{})

	createTables(engine)

	fmt.Println("Database running")
}

func createTables(e *xorm.Engine) {
	if err := e.Sync2(new(User)); err != nil {
		panic(err)
	}
	if err := e.Sync2(new(Message)); err != nil {
		panic(err)
	}
	if err := e.Sync2(new(Reaction)); err != nil {
		panic(err)
	}
	if err := e.Sync2(new(Respec)); err != nil {
		panic(err)
	}
	if err := e.Sync2(new(Mention)); err != nil {
		panic(err)
	}
}

func dbGetTotalRespec() (total int) {
	var user User

	temp, err := engine.SumInt(user, "Respec")
	if err != nil {
		panic(err)
	}

	total = int(temp)

	return
}

func dbGetUserRespec(discordUser *discordgo.User) (respec int) {
	user := &User{Username: discordUser.String(), ID: discordUser.ID}
	has, err := engine.Get(user)
	if err != nil {
		panic(err)
	}
	if has {
		respec = user.Respec
	}
	return
}

func dbLoadRespec(list *map[string]int) {
	var users []User
	if err := engine.Find(&users); err != nil {
		panic(err)
	}
	for _, v := range users {
		(*list)[v.Username] = v.Respec
	}
}

func dbGainRespec(discordUser *discordgo.User, respec int) {
	user := &User{Username: discordUser.String(), ID: discordUser.ID}
	has, err := engine.Get(user)
	if err != nil {
		panic(err)
	}
	if has {
		user.Respec += respec
		if _, err = engine.ID(core.PK{user.ID}).Cols("Respec").Update(user); err != nil {
			panic(err)
		}
	} else {
		user.Respec = respec
		if _, err = engine.Insert(user); err != nil {
			panic(err)
		}
	}
}

func dbNewMessage(discordUser *discordgo.User, message *discordgo.Message, numRespec int, timeStamp time.Time) {
	msg := &Message{ID: message.ID, Content: message.Content, Respec: numRespec, UserID: discordUser.String(), Time: timeStamp}
	if _, err := engine.Insert(msg); err != nil {
		panic(err)
	}
}

func dbMessageExists(messageID string) (has bool) {
	has, err := engine.Exist(Message{ID: messageID})
	if err != nil {
		panic(err)
	}
	return
}

func dbGiveRespec(giver *discordgo.User, receiver *discordgo.User, numRespec int, timeStamp time.Time) {
	respec := &Respec{GiverID: giver.String(), ReceiverID: receiver.String(), Respec: numRespec, Time: timeStamp}
	if _, err := engine.Insert(respec); err != nil {
		panic(err)
	}
}

func dbReactionAdd(discordUser *discordgo.User, rctn *discordgo.MessageReaction, timeStamp time.Time) {
	reaction := Reaction{MessageID: rctn.MessageID, UserID: discordUser.String(), Content: rctn.Emoji.ID}

	has, err := engine.Exist(&reaction)
	if err != nil {
		panic(err)
	}

	if has {
		if _, err = engine.Delete(&reaction); err != nil {
			panic(err)
		}
	}

	reaction.Time = timeStamp

	if _, err = engine.Insert(reaction); err != nil {
		panic(err)
	}
}

func dbReactionRemove(discordUser *discordgo.User, rctn *discordgo.MessageReaction, timeStamp time.Time) {
	reaction := Reaction{MessageID: rctn.MessageID, UserID: discordUser.String(), Content: rctn.Emoji.ID}

	has, err := engine.Get(&reaction)
	if err != nil {
		panic(err)
	}

	if has {
		if _, err = engine.ID(core.PK{reaction.Content, reaction.MessageID, reaction.UserID}).Cols("Removed").Update(Reaction{Removed: timeStamp}); err != nil {
			panic(err)
		}
	} else {
		reaction.Time = timeStamp
		reaction.Removed = timeStamp
		if _, err = engine.Insert(reaction); err != nil {
			panic(err)
		}
	}
}

func dbMention(giver *discordgo.User, receiver *discordgo.User, message *discordgo.Message, numRespec int, timeStamp time.Time) {
	mention := Mention{GiverID: giver.String(), ReceiverID: receiver.String(), MessageID: message.ID, Respec: numRespec, Time: timeStamp}
	if _, err := engine.Insert(mention); err != nil {
		panic(err)
	}
}

func dbGetLastMessages(list *map[string]time.Time) {
	var messages []Message

	err := engine.Alias("a").Select("a.UserID , a.Time").Where("a.Time = (SELECT max(Time) From Message b WHERE a.UserID = b.UserID)").Find(&messages)

	if err != nil {
		panic(err)
	}

	for _, v := range messages {
		(*list)[v.UserID] = v.Time
	}
}

func dbGetLastRespecs(list *map[string]time.Time) {
	var respec []Respec

	err := engine.Alias("a").Select("a.GiverID , a.Time").Where("a.Time = (SELECT max(Time) From Respec b WHERE a.GiverID = b.GiverID)").Find(&respec)

	if err != nil {
		panic(err)
	}

	for _, v := range respec {
		(*list)[v.GiverID] = v.Time
	}
}

func dbGetLastMentions(list *map[string]time.Time) {
	var mention []Mention

	err := engine.Alias("a").Select("a.ReceiverID , a.Time").Where("a.Time = (SELECT max(Time) From Mention b WHERE a.ReceiverID = b.ReceiverID)").Find(&mention)

	if err != nil {
		panic(err)
	}

	for _, v := range mention {
		(*list)[v.ReceiverID] = v.Time
	}
}

func dbGetLastReactionsAdded(list *map[reactionUsers]time.Time) {
	var rm []joinReactionMessage

	err := engine.Table("Reaction").Alias("r").Select("r.UserID, m.UserID, max(r.Time) AS Time").
		Join("INNER", []string{"Message", "m"}, "r.MessageID = m.ID").
		Where("r.Time = (SELECT max(Time) From Reaction b WHERE r.UserID = b.UserID AND m.ID = b.MessageID)").
		GroupBy("r.UserID, m.UserID").
		Find(&rm)

	if err != nil {
		panic(err)
	}

	for _, v := range rm {
		(*list)[reactionUsers{v.Reaction.UserID, v.Message.UserID}] = v.Reaction.Time
	}
}

func dbGetLastReactionsRemoved(list *map[reactionUsers]time.Time) {
	var rm []joinReactionMessage

	err := engine.Table("Reaction").Alias("r").Select("r.UserID, m.UserID, max(r.Removed) AS Removed").
		Join("INNER", []string{"Message", "m"}, "r.MessageID = m.ID").
		Where("r.Removed = (SELECT max(Removed) From Reaction b WHERE r.UserID = b.UserID AND m.ID = b.MessageID)").
		GroupBy("r.UserID, m.UserID").
		Find(&rm)

	if err != nil {
		panic(err)
	}

	for _, v := range rm {
		(*list)[reactionUsers{v.Reaction.UserID, v.Message.UserID}] = v.Reaction.Removed
	}
}

func purgeDB() error {
	engine.ShowSQL(true)
	var users []User
	var messages []Message
	var reactions []Reaction
	var respecs []Respec
	var Mention []Mention
	if err := engine.Find(&users); err != nil {
		return err
	}
	for _, v := range users {
		if _, err := engine.Delete(&v); err != nil {
			return err
		}
	}
	if err := engine.Find(&messages); err != nil {
		return err
	}
	for _, v := range messages {
		if _, err := engine.Delete(&v); err != nil {
			return err
		}
	}
	if err := engine.Find(&reactions); err != nil {
		return err
	}
	for _, v := range reactions {
		if _, err := engine.Delete(&v); err != nil {
			return err
		}
	}
	if err := engine.Find(&respecs); err != nil {
		return err
	}
	for _, v := range respecs {
		if _, err := engine.Delete(&v); err != nil {
			return err
		}
	}
	if err := engine.Find(&Mention); err != nil {
		return err
	}
	for _, v := range Mention {
		if _, err := engine.Delete(&v); err != nil {
			return err
		}
	}
	return nil

}
