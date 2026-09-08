get_payments() {
	psql ledger -c "SELECT * FROM payments;"
}
delete_payments() {
	psql ledger -c "DELETE FROM payments;"

}
get_loans() {
	psql ledger -c "SELECT * FROM loans;"
}
get_investments() {
	psql ledger -c "SELECT * FROM investments;"
}

get_overpayments() {
	psql ledger -c "SELECT * FROM overpayments;"
}
get_entries() {
	psql ledger -c "SELECT * FROM entries;"
}
get_clients() {
	psql ledger -c "SELECT * FROM clients;"
}
delete_clients() {
	psql ledger -c "DELETE FROM clients;"
}
get_accounts() {
	psql ledger -c "SELECT * FROM accounts;"
}
cdelete_accounts() {
	psql ledger -c "DELETE FROM accounts;"
}
get_payment_references() {
	psql ledger -c "SELECT * FROM payment_references;"
}
delete_entries() {
	psql ledger -c "DELETE FROM entries;"
}
delete_loans() {
	psql ledger -c "DELETE FROM loans;"
}
delete_investments() {
	psql ledger -c "DELETE FROM investments;"
}
delete_overpayments() {
	psql ledger -c "DELETE FROM overpayments;"
}
seed_clients() {
psql ledger -c "INSERT INTO clients (id, name, client_type) VALUES (gen_random_uuid(), 'John Doe', 'loan'), (gen_random_uuid(), 'Jane Smith', 'investment');"
}
seed_accounts() {
psql ledger -c "INSERT INTO accounts (id, name, type, client_id) VALUES (gen_random_uuid(), 'LOAN-2026', 'loan', (SELECT id FROM clients WHERE name = 'John Doe')), (gen_random_uuid(), 'INV-2026', 'investment', (SELECT id FROM clients WHERE name = 'Jane Smith'));"
}
seed_payment_references() {
	psql ledger -c "INSERT INTO payment_references (id, reference, entity_type) VALUES (gen_random_uuid(), 'LOAN-2026', 'loan'), (gen_random_uuid(), 'INV-2026', 'investment');"
}

if [ "$1" = "payments" ]; then
	get_payments
elif [ "$1" = "delete_payments" ]; then
	delete_payments
elif [ "$1" = "loans" ]; then
	get_loans
elif [ "$1" = "investments" ]; then
	get_investments
elif [ "$1" = "overpayments" ]; then
	get_overpayments
elif [ "$1" = "entries" ]; then
	get_entries
elif [ "$1" = "delete_entries" ]; then
	delete_entries
elif [ "$1" = "delete_loans" ]; then
	delete_loans
elif [ "$1" = "delete_investments" ]; then
	delete_investments
elif [ "$1" = "delete_overpayments" ]; then
	delete_overpayments
elif [ "$1" = "seed_clients" ]; then
	seed_clients
elif [ "$1" = "seed_accounts" ]; then
	seed_accounts
elif [ "$1" = "clients" ]; then
	get_clients
elif [ "$1" = "accounts" ]; then
	get_accounts
elif [ "$1" = "payment_references" ]; then
	get_payment_references
elif [ "$1" = "seed_payment_references" ]; then
	seed_payment_references
elif [ "$1" = "delete_accounts" ]; then
	delete_accounts
elif [ "$1" = "delete_clients" ]; then
	delete_clients
else
	echo "Usage: $0 ledger|payments|delete_payments|loans|investments|overpayments|entries|delete_entries|delete_loans|delete_investments|delete_overpayments|seed_clients|seed_accounts"
fi
