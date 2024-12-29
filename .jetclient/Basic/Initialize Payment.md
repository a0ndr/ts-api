```toml
name = 'Initialize Payment'
method = 'POST'
url = '{{URI}}/payments/init'
sortWeight = 4000000
id = 'ad6ab171-b834-45c2-bfef-b183e4468b6e'

[body]
type = 'JSON'
raw = '''
{
  "amount": "1",
  "currency": "EUR",
  "debtorIban": "SK1911000000001913888635",
  "creditorIban": "SK4011000000004145138420",
  "creditorName": ":3",
  "note": ":3",
  "type": "sepa-credit-transfers"
}'''
```
