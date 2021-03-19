#      Warning!!!!!!!!!!!This file is readonly!Don't modify this file!

help() {
	echo "cmd.sh — every thing you need"
	echo "         please install golang"
	echo ""
	echo "Usage:"
	echo "   ./cmd.sh <option>"
	echo ""
	echo "Options:"
	echo "   run                       Run this program"
	echo "   build                     Complie this program to binary"
	echo "   h/-h/help/-help/--help    Show this message"
}

run() {
	go mod tidy
	go run main.go
}

build() {
	go mod tidy
	go build -ldflags "-s -w" -o main
	if (type upx >/dev/null 2>&1);then
		upx -9  main
	else
		echo "recommand to use upx to compress exec file"
	fi
}

if !(type go >/dev/null 2>&1);then
	echo "missing dependence: golang"
	exit 0
fi

if [[ $# == 0 ]] || [[ "$1" == "h" ]] || [[ "$1" == "help" ]] || [[ "$1" == "-h" ]] || [[ "$1" == "-help" ]] || [[ "$1" == "--help" ]]; then
	help
	exit 0
fi

if [[ "$1" == "run" ]]; then
	run
	exit 0
fi

if [[ "$1" == "build" ]];then
	build
	exit 0
fi

echo "option unsupport"
help
