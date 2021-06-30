package bus

var channels channelsStore
var started bool = false

type channelsStore struct {
	updatesChannel     chan Update
	accountChannel     chan MonetaryAccountDocument
	transactionChannel chan TransactionDocument
	scheduleChannel    chan ScheduleDocument
	directDebitChannel chan DirectDebitTransactionDocument
}

func UpdatesChannelForReading() <-chan Update {
	ensureStarted()
	return channels.updatesChannel
}

func UpdatesChannelForWriting() chan<- Update {
	ensureStarted()
	return channels.updatesChannel
}

func AccountChannelForReading() <-chan MonetaryAccountDocument {
	ensureStarted()
	return channels.accountChannel
}

func AccountChannelForWriting() chan<- MonetaryAccountDocument {
	ensureStarted()
	return channels.accountChannel
}

func TransactionChannelForReading() <-chan TransactionDocument {
	ensureStarted()
	return channels.transactionChannel
}

func TransactionChannelForWriting() chan<- TransactionDocument {
	ensureStarted()
	return channels.transactionChannel
}

func ScheduleChannelForReading() <-chan ScheduleDocument {
	ensureStarted()
	return channels.scheduleChannel
}

func ScheduleChannelForWriting() chan<- ScheduleDocument {
	ensureStarted()
	return channels.scheduleChannel
}

func DirectDebitChannelForReading() <-chan DirectDebitTransactionDocument {
	ensureStarted()
	return channels.directDebitChannel
}

func DirectDebitChannelForWriting() chan<- DirectDebitTransactionDocument {
	ensureStarted()
	return channels.directDebitChannel
}

func ensureStarted() {
	if started {
		return
	}

	channels = channelsStore{
		updatesChannel:     make(chan Update, 50),
		accountChannel:     make(chan MonetaryAccountDocument, 50),
		transactionChannel: make(chan TransactionDocument, 50),
		directDebitChannel: make(chan DirectDebitTransactionDocument, 50),
		scheduleChannel:    make(chan ScheduleDocument, 50),
	}

	started = true
}
