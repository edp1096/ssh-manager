# SSH 관리자

PuTTY 대신 쓸 목적

## 작업
* 웹 - 관리화면은 웹종류로 쓴다. ~~wails 봐야됨~~. spawn만 해도 되면 server는 ~~없어도 되겠다.~~ 있어야 된다
* 순수 터미널 - 화면 껍데기로는 터미널을 쓴다.
* ssh 클라이언트 - 비번입력 문제 때문에 직접 만든다. cli용 일단 완성

## 화면
* 윈도우는 windows terminal
    * https://learn.microsoft.com/ko-kr/windows/terminal/command-line-arguments?tabs=windows#split-pane-command
    * `wt -w 0 sp` - 판넬 분할만 되고 명령 실행은 똑바로 안된다.
* 리눅스는 terminator only
    * 터미널 실행기 확인 - `ps -o 'cmd=' -p $(ps -o 'ppid=' -p $$)`
    * https://github.com/gnome-terminator/terminator/issues/446#issuecomment-886137668
    * https://askubuntu.com/questions/640096/how-do-i-check-which-terminal-i-am-using/640112#640112
