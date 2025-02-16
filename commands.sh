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

if [ "$1" = "generate-sorted" ]; then
    cd ~/git/6.5840-golabs-2025/src/main/mr-tmp

    for file in m-out-*; do
        # check if the file exists
        if [[ -f "$file" ]]; then
            sorted_file="${file}-sorted"
            sort "$file" > "$sorted_file"
        fi
    done

    for file in mr-out-*; do
        # check if the file exists
        if [[ -f "$file" ]]; then
            sorted_file="${file}-sorted"
            sort "$file" > "$sorted_file"
        fi
    done
fi


