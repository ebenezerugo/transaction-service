package kafka

const (
	TopicTransactions       = "transactions"
	TopicTransactionEvents  = "transaction.events"
	TopicTransferCommands   = "transfer.commands"
	TopicDepositCommands    = "deposit.commands"
	TopicWithdrawalCommands = "withdrawal.commands"
	TopicHoldCommands       = "hold.commands"
	TopicReleaseCommands    = "release.commands"
	TopicUndoCommands       = "undo.commands"
	TopicAdjustCommands     = "adjust.commands"
	TopicDLQ                = "transactions.dlq"
	TopicBalanceUpdates     = "balance.updates"
)
