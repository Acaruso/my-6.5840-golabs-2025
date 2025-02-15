if [ "$1" = "test" ]; then
    cd ~/git/6.5840-golabs-2025/src/main
    bash test-mr.sh
fi

if [ "$1" = "test-wc" ]; then
    cd ~/git/6.5840-golabs-2025/src/main
    bash test-mr-wc.sh
fi

if [ "$1" = "clean" ]; then
    cd ~/git/6.5840-golabs-2025/src/main
    rm *-map-output
fi
