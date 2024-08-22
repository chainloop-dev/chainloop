package main

import future.keywords.in

violations[msg] {
    not made_with_syft

    msg := "Not made with syft"
}

made_with_syft {
    some creator in input.creationInfo.creators
    contains(creator, "syft")
}