package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInsufficientFunds   = errors.New("недостаточно средств на счете")
	ErrInvalidAmount       = errors.New("некорректная сумма")
	ErrAccountNotFound     = errors.New("счет не найден")
	ErrSameAccountTransfer = errors.New("невозможно перевод на тот же счет")
)

type Transaction struct {
	Timestamp   time.Time
	Type        string
	Amount      float64
	From        string
	To          string
	Description string
}

type Account struct {
	ID           string
	Owner        string
	Balance      float64
	Transactions []Transaction
}

type AccountService interface {
	Deposit(amount float64) error
	Withdraw(amount float64) error
	Transfer(to *Account, amount float64) error
	GetBalance() float64
	GetStatement() string
}

type Storage interface {
	SaveAccount(account *Account) error
	LoadAccount(accountID string) (*Account, error)
	GetAllAccounts() ([]*Account, error)
}

type MemoryStorage struct {
	accounts map[string]*Account
	mutex    sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		accounts: make(map[string]*Account),
	}
}

func (ms *MemoryStorage) SaveAccount(account *Account) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.accounts[account.ID] = account
	return nil
}

func (ms *MemoryStorage) LoadAccount(accountID string) (*Account, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	account, exists := ms.accounts[accountID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (ms *MemoryStorage) GetAllAccounts() ([]*Account, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	accounts := make([]*Account, 0, len(ms.accounts))
	for _, acc := range ms.accounts {
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func (acc *Account) Deposit(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	acc.Balance += amount
	acc.Transactions = append(acc.Transactions, Transaction{
		Timestamp:   time.Now(),
		Type:        "ПОПОЛНЕНИЕ",
		Amount:      amount,
		To:          acc.ID,
		Description: fmt.Sprintf("Пополнение счета на %.2f", amount),
	})
	return nil
}

func (acc *Account) Withdraw(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if acc.Balance < amount {
		return ErrInsufficientFunds
	}

	acc.Balance -= amount
	acc.Transactions = append(acc.Transactions, Transaction{
		Timestamp:   time.Now(),
		Type:        "СНЯТИЕ",
		Amount:      amount,
		From:        acc.ID,
		Description: fmt.Sprintf("Снятие со счета %.2f", amount),
	})
	return nil
}

func (acc *Account) Transfer(to *Account, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if acc.Balance < amount {
		return ErrInsufficientFunds
	}
	if acc.ID == to.ID {
		return ErrSameAccountTransfer
	}

	acc.Balance -= amount
	to.Balance += amount

	acc.Transactions = append(acc.Transactions, Transaction{
		Timestamp:   time.Now(),
		Type:        "ПЕРЕВОД",
		Amount:      amount,
		From:        acc.ID,
		To:          to.ID,
		Description: fmt.Sprintf("Перевод на счет %s: %.2f", to.ID, amount),
	})

	to.Transactions = append(to.Transactions, Transaction{
		Timestamp:   time.Now(),
		Type:        "ЗАЧИСЛЕНИЕ",
		Amount:      amount,
		From:        acc.ID,
		To:          to.ID,
		Description: fmt.Sprintf("Перевод от счета %s: %.2f", acc.ID, amount),
	})

	return nil
}

func (acc *Account) GetBalance() float64 {
	return acc.Balance
}

func (acc *Account) GetStatement() string {
	if len(acc.Transactions) == 0 {
		return fmt.Sprintf("Выписка по счету %s\nВладелец: %s\nБаланс: %.2f\nИстория транзакций: нет операций\n",
			acc.ID, acc.Owner, acc.Balance)
	}

	statement := fmt.Sprintf("ВЫПИСКА ПО СЧЕТУ %s\n", acc.ID)
	statement += fmt.Sprintf("Владелец: %s\n", acc.Owner)
	statement += fmt.Sprintf("Текущий баланс: %.2f\n", acc.Balance)
	statement += "─────────────────────────────────────\n"
	statement += "ДАТА И ВРЕМЯ        | ТИП ОПЕРАЦИИ | СУММА  | ОПИСАНИЕ\n"
	statement += "─────────────────────────────────────\n"

	for _, tx := range acc.Transactions {
		statement += fmt.Sprintf("%s | %-12s | %6.2f | %s\n",
			tx.Timestamp.Format("02.01.2006 15:04"),
			tx.Type,
			tx.Amount,
			tx.Description)
	}
	return statement
}

func showMainMenu() {
	fmt.Println("\n═══════════════════════════════════")
	fmt.Println("           ГЛАВНОЕ МЕНЮ")
	fmt.Println("═══════════════════════════════════")
	fmt.Println("1. Создать новый счет")
	fmt.Println("2. Работа с существующим счетом")
	fmt.Println("3. Список всех счетов")
	fmt.Println("4. Выйти")
	fmt.Print("Выберите опцию: ")
}

func createAccount(store *MemoryStorage, scanner *bufio.Scanner) {
	fmt.Println("\n--- СОЗДАНИЕ НОВОГО СЧЕТА ---")
	fmt.Print("Введите имя владельца счета: ")
	scanner.Scan()
	owner := strings.TrimSpace(scanner.Text())

	if owner == "" {
		fmt.Println("❌ Имя владельца не может быть пустым")
		return
	}

	accounts, _ := store.GetAllAccounts()
	newID := fmt.Sprintf("ACC%04d", len(accounts)+1)

	account := &Account{
		ID:      newID,
		Owner:   owner,
		Balance: 0,
	}

	err := store.SaveAccount(account)
	if err != nil {
		fmt.Println("❌ Ошибка при создании счета:", err)
		return
	}

	fmt.Printf("✅ Счет создан успешно!\n")
	fmt.Printf("   ID счета: %s\n", newID)
	fmt.Printf("   Владелец: %s\n", owner)
	fmt.Printf("   Начальный баланс: 0.00\n")
}

func selectAccountMenu(store *MemoryStorage, scanner *bufio.Scanner) {
	fmt.Println("\n--- ВЫБОР СЧЕТА ---")
	fmt.Print("Введите ID счета: ")
	scanner.Scan()
	accountID := strings.TrimSpace(scanner.Text())

	account, err := store.LoadAccount(accountID)
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	fmt.Printf("✅ Счет найден: %s (%s)\n", account.ID, account.Owner)
	accountOperations(store, scanner, account)
}

func accountOperations(store *MemoryStorage, scanner *bufio.Scanner, account *Account) {
	for {
		fmt.Println("\n═══════════════════════════════════")
		fmt.Printf("СЧЕТ: %s | Владелец: %s\n", account.ID, account.Owner)
		fmt.Printf("Баланс: %.2f\n", account.GetBalance())
		fmt.Println("═══════════════════════════════════")
		fmt.Println("1. Пополнить счет")
		fmt.Println("2. Снять средства")
		fmt.Println("3. Перевести другому счету")
		fmt.Println("4. Просмотреть выписку")
		fmt.Println("5. Вернуться в главное меню")
		fmt.Print("Выберите опцию: ")

		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "1":
			deposit(store, scanner, account)
		case "2":
			withdraw(store, scanner, account)
		case "3":
			transfer(store, scanner, account)
		case "4":
			getStatement(account)
		case "5":
			return
		default:
			fmt.Println("❌ Неверная опция")
		}
	}
}

func deposit(store *MemoryStorage, scanner *bufio.Scanner, account *Account) {
	amount, err := getAmount(scanner, "Введите сумму для пополнения: ")
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	err = account.Deposit(amount)
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	store.SaveAccount(account)
	fmt.Printf("✅ Пополнение на %.2f прошло успешно\n", amount)
	fmt.Printf("   Новый баланс: %.2f\n", account.GetBalance())
}

func withdraw(store *MemoryStorage, scanner *bufio.Scanner, account *Account) {
	amount, err := getAmount(scanner, "Введите сумму для снятия: ")
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	err = account.Withdraw(amount)
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	store.SaveAccount(account)
	fmt.Printf("✅ Снятие %.2f прошло успешно\n", amount)
	fmt.Printf("   Новый баланс: %.2f\n", account.GetBalance())
}

func transfer(store *MemoryStorage, scanner *bufio.Scanner, fromAccount *Account) {
	fmt.Print("Введите ID счета получателя: ")
	scanner.Scan()
	toAccountID := strings.TrimSpace(scanner.Text())

	if fromAccount.ID == toAccountID {
		fmt.Println("❌ Ошибка:", ErrSameAccountTransfer)
		return
	}

	amount, err := getAmount(scanner, "Введите сумму для перевода: ")
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	toAccount, err := store.LoadAccount(toAccountID)
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	err = fromAccount.Transfer(toAccount, amount)
	if err != nil {
		fmt.Println("❌ Ошибка:", err)
		return
	}

	store.SaveAccount(fromAccount)
	store.SaveAccount(toAccount)
	fmt.Printf("✅ Перевод на сумму %.2f выполнен успешно\n", amount)
	fmt.Printf("   Получатель: %s (%s)\n", toAccount.ID, toAccount.Owner)
	fmt.Printf("   Новый баланс: %.2f\n", fromAccount.GetBalance())
}

func getStatement(account *Account) {
	fmt.Println("\n" + account.GetStatement())
}

func listAllAccounts(store *MemoryStorage) {
	accounts, err := store.GetAllAccounts()
	if err != nil {
		fmt.Println("❌ Ошибка при получении списка счетов:", err)
		return
	}

	if len(accounts) == 0 {
		fmt.Println("📝 Созданных счетов нет")
		return
	}

	fmt.Println("\n--- СПИСОК ВСЕХ СЧЕТОВ ---")
	for i, acc := range accounts {
		fmt.Printf("%d. %s - %s (Баланс: %.2f)\n",
			i+1, acc.ID, acc.Owner, acc.GetBalance())
	}
}

func getAmount(scanner *bufio.Scanner, prompt string) (float64, error) {
	fmt.Print(prompt)
	scanner.Scan()
	amountStr := strings.TrimSpace(scanner.Text())

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		return 0, ErrInvalidAmount
	}

	return amount, nil
}

func main() {
	store := NewMemoryStorage()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("=== БАНКОВСКОЕ ПРИЛОЖЕНИЕ ===")
	fmt.Println("Добро пожаловать в банковскую систему!")

	for {
		showMainMenu()

		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "1":
			createAccount(store, scanner)
		case "2":
			selectAccountMenu(store, scanner)
		case "3":
			listAllAccounts(store)
		case "4":
			fmt.Println("Выход из приложения. До свидания!")
			return
		default:
			fmt.Println("❌ Неверная опция. Пожалуйста, выберите от 1 до 4")
		}
	}
}
