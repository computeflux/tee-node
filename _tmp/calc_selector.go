package main

import (
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

func main() {
	s1 := model.MethodToSelector("store_encrypted_auth_user")
	fmt.Println("store_encrypted_auth_user:", hex.EncodeToString(s1[:]))
	s2 := model.MethodToSelector("store_encrypted_private_key")
	fmt.Println("store_encrypted_private_key:", hex.EncodeToString(s2[:]))
	s3 := model.MethodToSelectorWithMutates("get_encrypted_auth_user", false)
	fmt.Println("get_encrypted_auth_user:", hex.EncodeToString(s3[:]))
	s4 := model.MethodToSelectorWithMutates("get_encrypted_private_key", false)
	fmt.Println("get_encrypted_private_key:", hex.EncodeToString(s4[:]))
}
