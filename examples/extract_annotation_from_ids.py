import datetime

with open('test.ids') as f:
    lines = f.readlines()
    for row in lines:
        date = row.split('|')[2]
        filtrationstep = (datetime.datetime.strptime(date, "%Y-%m-%d")-datetime.datetime(2019,12,30)).days
        print(filtrationstep)
