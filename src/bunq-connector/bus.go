package bunqconnector

import (
	"app/bus"
	"app/primitives"
	"strconv"

	"github.com/Rhymond/go-money"
	"github.com/almerlucke/go-iban/iban"
)

type integrationChannels interface {
	updatesChannel() chan<- bus.Update
	accountChannel() chan<- bus.MonetaryAccountDocument
	transactionChannel() chan<- bus.TransactionDocument
	scheduleChannel() chan<- bus.ScheduleDocument
	directDebitChannel() chan<- bus.DirectDebitTransactionDocument
}

type busChannels struct {
}

func newBusChannels() busChannels {
	return busChannels{}
}

func (b busChannels) updatesChannel() chan<- bus.Update {
	return bus.UpdatesChannelForWriting()
}

func (b busChannels) accountChannel() chan<- bus.MonetaryAccountDocument {
	return bus.AccountChannelForWriting()
}

func (b busChannels) transactionChannel() chan<- bus.TransactionDocument {
	return bus.TransactionChannelForWriting()
}

func (b busChannels) scheduleChannel() chan<- bus.ScheduleDocument {
	return bus.ScheduleChannelForWriting()
}

func (b busChannels) directDebitChannel() chan<- bus.DirectDebitTransactionDocument {
	return bus.DirectDebitChannelForWriting()
}

func (account apiAccount) mapToDocument(userID primitives.UserID) bus.MonetaryAccountDocument {
	return bus.MonetaryAccountDocument{
		Iban:                account.iban,
		Joint:               account.joint,
		OwnerUserID:         userID,
		Alias:               account.alias,
		Institution:         primitives.Bunq,
		InstitutionEntityID: strconv.Itoa(int(account.bunqAccountID)),
		Balance:             account.balance,
		FetchTimestamp:      account.fetchTimestamp,
	}
}

func (tx apiTransaction) mapToDocument() bus.TransactionDocument {
	var geolocation *bus.Geolocation
	if tx.geolocation != nil {
		geolocation = &bus.Geolocation{
			Latitude:  tx.geolocation.latitude,
			Longitude: tx.geolocation.longitude,
			Radius:    tx.geolocation.radius,
			Altitude:  tx.geolocation.altitude,
		}
	}

	var fromName *string
	var fromIban *iban.IBAN
	var fromInstitutionEntityID *string
	var fromInstitution *primitives.Institution

	var toName *string
	var toIban *iban.IBAN
	var toInstitutionEntityID *string
	var toInstitution *primitives.Institution

	if (tx.amount != money.Money{}) && (tx.amount.IsPositive()) {
		fromName = tx.counterpartyName
		fromIban = tx.counterpartyIban

		toName = tx.name
		toIban = tx.iban
		x := strconv.Itoa(int(tx.bunqTransactionID))
		toInstitutionEntityID = &x
		y := primitives.Bunq
		toInstitution = &y
	} else if (tx.amount != money.Money{}) {
		fromName = tx.name
		fromIban = tx.iban
		x := strconv.Itoa(int(tx.bunqTransactionID))
		fromInstitutionEntityID = &x
		y := primitives.Bunq
		fromInstitution = &y

		toName = tx.counterpartyName
		toIban = tx.counterpartyIban
	}

	return bus.TransactionDocument{
		Amount: tx.amount,

		FromName:                fromName,
		FromIBAN:                fromIban,
		FromInstition:           fromInstitution,
		FromInstitutionEntityID: fromInstitutionEntityID,

		ToName:              toName,
		ToIBAN:              toIban,
		ToInstition:         toInstitution,
		ToInstitionEntityID: toInstitutionEntityID,

		Description:           tx.description,
		InstitutionScheduleID: tx.institutionScheduleID,
		BalanceAfterMutation:  tx.balanceAfterMutation,
		Geolocation:           geolocation,
		TransactionDate:       tx.transactionDate,
		FetchTimestamp:        tx.fetchTimestamp,
	}
}

func (schedule apiSchedule) mapToDocument() bus.ScheduleDocument {
	var fromName string
	var fromIban iban.IBAN

	var toName string
	var toIban iban.IBAN

	if (schedule.amount != money.Money{}) && (schedule.amount.IsPositive()) {
		fromName = schedule.counterpartyName
		fromIban = schedule.counterpartyIBAN

		toName = schedule.name
		toIban = schedule.iban
	} else if (schedule.amount != money.Money{}) {
		fromName = schedule.name
		fromIban = schedule.iban

		toName = schedule.counterpartyName
		toIban = schedule.counterpartyIBAN
	}

	return bus.ScheduleDocument{
		Institution:         primitives.Bunq,
		InstitutionEntityID: strconv.Itoa(int(schedule.bunqScheduleID)),
		FromIBAN:            fromIban,
		FromName:            fromName,
		ToName:              toName,
		ToIBAN:              toIban,
		Frequency:           schedule.frequency,
		StartDate:           schedule.startDate,
		EndDate:             schedule.endDate,
		Amount:              schedule.amount,
		Description:         schedule.description,
		FetchTimestamp:      schedule.fetchTimestamp,
	}
}

func (directDebit apiDirectDebitTransaction) mapToDocument() bus.DirectDebitTransactionDocument {
	var fromName string
	var fromIban iban.IBAN

	var toName string
	var toIban iban.IBAN

	if (directDebit.amount != money.Money{}) && (directDebit.amount.IsPositive()) {
		fromName = directDebit.counterpartyName
		fromIban = directDebit.counterpartyIBAN

		toName = directDebit.name
		toIban = directDebit.iban
	} else if (directDebit.amount != money.Money{}) {
		fromName = directDebit.name
		fromIban = directDebit.iban

		toName = directDebit.counterpartyName
		toIban = directDebit.counterpartyIBAN
	}

	return bus.DirectDebitTransactionDocument{
		Institution:         primitives.Bunq,
		InstititionEntityID: strconv.Itoa(int(directDebit.bunqDirectDebitTransactionID)),
		FromIBAN:            fromIban,
		FromName:            fromName,
		ToIBAN:              toIban,
		ToName:              toName,
		Description:         directDebit.description,
		CreditSchemeID:      directDebit.creditSchemeID,
		MandateID:           directDebit.mandateID,
		TransactionDate:     directDebit.created,
		Amount:              directDebit.amount,
		FetchTimestamp:      directDebit.fetchTimestamp,
	}
}

func newDoneUpdate(start bus.StartRefreshUpdate) bus.DoneRefreshingUpdate {
	return bus.NewDoneRefreshingUpdateFrom(start)
}
