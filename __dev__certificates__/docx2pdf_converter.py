from docx2pdf import convert
import os
import sys

# ------- HELP FUNCTIONS ------- #
def getLineAndPath(line):
    line = line.strip().split(' {')[0] # "отрезаем" предыдущие ошибки
    path = line.split('=')[1] # "отрезаем" конструкцию вида "CERTIFICATES_PATH="
    path = path[1:len(path) - 1] # "отрезаем" кавычки от строки вида "..."
    return line, path

def writeLogAndExit(old_file, new_file, log):
    print(log, file = new_file, end = '')
    old_file.close()
    os.remove(OLD_FILE_NAME)
    sys.exit(7)
# ------------------------------ #

# --------- MAIN SCRIPT --------- #
PY_SCRIPT_PATH = os.path.dirname(os.path.realpath(sys.argv[0])) # нельзя os.getcwd(), т.к. python-скрипт запускается из go-скрипта => os.getcwd() вернет расположение go-скрипта
OLD_FILE_NAME = os.path.join(PY_SCRIPT_PATH, '.old_path')
NEW_FILE_NAME = os.path.join(PY_SCRIPT_PATH, '.path')

if os.path.isfile(NEW_FILE_NAME):
    os.rename(NEW_FILE_NAME, OLD_FILE_NAME)
else:    
   sys.exit(7) # почему-то go-скрипт не создал '.path' файл


with open(NEW_FILE_NAME, 'w', encoding='utf-8') as new_file, open(OLD_FILE_NAME, 'r', encoding='utf-8') as old_file:
    for line in old_file:
        if line.startswith('CERTIFICATES_PATH'):
            try:
                line, path = getLineAndPath(line)
                if os.path.exists(path):
                    convert(path)
                    print(line, file = new_file, end = '') # запись в '.path' файл после convert(...) функции, т.к. если в ней ошибка, то сработает блок except, где будет нужная запись ошибки
                else:
                    log = f'{line} {{error: несуществующий путь к сертификатам}}'
                    writeLogAndExit(old_file, new_file, log) # почему-то go-скрипт записал '.path' файл несуществующий путь к сертификатам
            except Exception as error:
                errMes = 'an error occured' if not str(error) else f'error: {error}'
                log = f'{line} {{{errMes}}}'
                writeLogAndExit(old_file, new_file, log)

os.remove(OLD_FILE_NAME)
# ------------------------------- #