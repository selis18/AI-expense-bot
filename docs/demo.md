# Demonstration

Start the application with the settings described in README. Use two Telegram accounts to demonstrate ownership. The examples below are Russian because the bot replies in Russian.

1. From the first account, send `кофе 350`, then `такси 1200 вчера`. Check the amounts, dates, and owner in the panel.
2. Send `кофе`, answer the clarification with `250`. Check that only one additional expense appears.
3. Send `покажи траты за неделю`, then `сколько потратил на еду в этом месяце`. Compare the result with the panel filtered to that participant and period.
4. Send `последнее было не 250, а 450`. Check the updated expense in both interfaces.
5. Add two taxi expenses and send `удали такси`. Choose a list number, then answer `нет`. Repeat and answer `да`; check that only the selected expense is deleted.
6. From the second account, add an expense and request the first account's expense by ID. The bot must not expose or modify it. Both accounts' expenses remain visible in the panel.
7. Create, edit, and delete an expense in the panel. Apply participant, date, and category filters and compare the totals.
8. Restart the application. Existing expenses and an unexpired deletion confirmation must survive.

Automated coverage includes duplicate Telegram update processing, concurrent attempts to process the same update, rollback, ownership restrictions, expired history and confirmations, changed records during confirmation, invalid model arguments, Telegram errors, and browser CRUD. Live provider behaviour still needs the configured credentials; these steps are an acceptance procedure, not a claim that live APIs have already passed.

The implementation was developed with an AI assistant. Generated changes were checked with Go tests, PostgreSQL integration tests, real HTTP clients against local test servers, TypeScript compilation, browser automation against the application, and desktop/mobile screenshot inspection. The assistant did not receive API keys or publish the repository.
