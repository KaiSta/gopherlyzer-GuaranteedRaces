@echo off

::echo AVRORA 1/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\avrora.log > .\sshbresults\detailed3\avrora.log
::echo BATIK 2/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\batikNT.log > .\sshbresults\detailed3\batik.log
::echo H2 4/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\h2NT.log -n 50000000 > .\sshbresults\detailed3\h2.log
::echo LUSEARCH 6/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\lusearchNT.log -n 50000000 > .\sshbresults\detailed3\lusearch.log
::echo MONTE 8/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\montecarloNT.log -n 50000000 > .\sshbresults\detailed3\monte.log
::echo PMD 9/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\pmdNT.log -n 50000000 > .\sshbresults\detailed3\pmd.log
::echo SERIESC 10/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\seriesCNT.log -n 50000000 > .\sshbresults\detailed3\seriesc.log
::echo SOR 11/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\sorNT.log -n 50000000 > .\sshbresults\detailed3\sor.log
::echo XALAN 13/14
::go run main.go -mode sshbALLPARA -parser java -trace E:\traces\xalanFull.log -filter=1 > .\sshbresults\detailed3\xalan.log
::echo SUNFLOW 12/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\sunflowNTABORTED.log -n 50000000 > .\sshbresults\detailed3\sunflow.log
::echo CRYPT 3/14
::go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\cryptNT.log -n 50000000 > .\sshbresults\detailed3\crypt.log
echo LUFACT 5/14
go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\lufactNT.log -n 50000000 > .\sshbresults\detailed3\lufact.log
echo MOLDYN 7/14
go run main.go -mode sshbALLPARA -parser javainc -trace E:\traces\moldyn.log -n 50000000 > .\sshbresults\detailed3\moldyn.log
::echo TOMCAT 14/14
::go run main.go -mode sshbALLPARA -parser java -trace E:\traces\tomcatFull.log -filter=1 > .\sshbresults\detailed3\tomcat.log
