Output images/demo.gif
Set FontSize 15
Set Width 1150
Set Height 640
Set Padding 12
Set TypingSpeed 40ms
Set Framerate 15

Type "./sqltui examples/employees.csv examples/demo.db"
Enter
Sleep 2s

# navigate
Type "jjjjj"
Sleep 800ms

# row detail sheet
Enter
Sleep 1.6s
Type "q"
Sleep 600ms

# fuzzy search
Type "/"
Sleep 600ms
Type "eng"
Sleep 1.2s
Enter
Sleep 1.2s
Type "q"
Sleep 600ms

# inline filter via palette
Type ":"
Sleep 700ms
Type "f "
Sleep 500ms
Type "salary > 150000"
Sleep 700ms
Enter
Sleep 1.6s
Type "q"
Sleep 600ms

# full SQL query -> new tab
Type ":"
Sleep 700ms
Type "q "
Sleep 500ms
Type "select department, count(*) as headcount, cast(avg(salary) as int) as avg_pay from employees group by department order by avg_pay desc "
Sleep 900ms
Enter
Sleep 2s

# tab switcher
Type "t"
Sleep 1.4s
Escape
Sleep 400ms
Type "H"
Sleep 600ms
Type "H"
Sleep 600ms
Type "H"
Sleep 700ms

# histogram
Type ":"
Sleep 700ms
Type "histogram"
Sleep 500ms
Enter
Sleep 900ms
Down 5
Sleep 500ms
Enter
Sleep 600ms
Enter
Sleep 2.2s
Type "q"
Sleep 600ms

# theme live preview
Type ":"
Sleep 700ms
Type "theme"
Sleep 500ms
Enter
Sleep 900ms
Down 2
Sleep 1.4s
Down 2
Sleep 1.4s
Escape
Sleep 900ms

Type "Q"
Sleep 800ms
