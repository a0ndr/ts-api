```toml
name = 'Authorize'
method = 'POST'
url = '{{URI}}/authorize'
sortWeight = 1000000
id = '9eeacc15-7d1a-4ada-9188-5f74a870ed7a'

[body]
type = 'JSON'
raw = '''
{
  "companyId": "{{companyId}}"
}'''
```
