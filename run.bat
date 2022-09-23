//--------------------------------------------
//
//   ../consenesusPBFT/
//                    + src
//                    +----/2000/ b.exe
//                    +----/3000/ b.exe
//                    +----/4000/ b.exe
//                    +----/5000/ b.exe
                      +----/6000/ b.exe
//
//--------------------------------------------
del *.exe
go build
rename consensusPBFT.exe b.exe
del .\test\4000\*.exe
del .\test\4001\*.exe
del .\test\5000\*.exe
del .\test\5001\*.exe
del .\test\6000\*.exe
copy b.exe .\4000\
copy b.exe .\4001\
copy b.exe .\5000\
copy b.exe .\5001\
copy b.exe .\6000\
cd 4000
start cmd.exe /k "b.exe P1"
cd ../4001
start cmd.exe /k "b.exe P2"
cd ../5000
start cmd.exe /k "b.exe P3"
cd ../5001
start cmd.exe /k "b.exe P4"
cd ../6000
start cmd.exe /k "b.exe P5"