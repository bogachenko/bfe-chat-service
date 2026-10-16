package main

import (
 "context"
 "crypto"
 "crypto/ed25519"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "flag"
 "fmt"
 "os"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 jose "github.com/go-jose/go-jose/v4"
)
func main(){
 socket:=flag.String("socket","","BFE Drive UDS")
 token:=flag.String("token","opaque-session-token","test token")
 expect:=flag.String("expect","ok","ok, invalid, unavailable or jkt")
 flag.Parse()
 seed:=sha256.Sum256([]byte("dp-m01-independent-fixture-private-key"))
 priv:=ed25519.NewKeyFromSeed(seed[:])
 pub:=priv.Public().(ed25519.PublicKey)
 thumb,err:=(&jose.JSONWebKey{Key:pub}).Thumbprint(crypto.SHA256)
 must(err)
 if *expect=="jkt"{fmt.Print(base64.RawURLEncoding.EncodeToString(thumb));return}
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:priv,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
 must(err)
 ath:=sha256.Sum256([]byte(*token))
 claims,err:=json.Marshal(map[string]any{"jti":fmt.Sprintf("qa-%d",time.Now().UnixNano()),"iat":time.Now().Unix(),"htm":"GET","htu":"https://chat.example.test/connection","ath":base64.RawURLEncoding.EncodeToString(ath[:])});must(err)
 signed,err:=signer.Sign(claims);must(err)
 proof,err:=signed.CompactSerialize();must(err)
 v,err:=authentication.NewValidator(*socket,5*time.Minute);must(err)
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 caller,e:=v.ValidateCaller(ctx,*token,proof,authentication.RequestBinding{Method:"GET",Target:"https://chat.example.test/connection"})
 switch *expect{
 case "ok":if e!=nil||caller.AccountID==""||caller.AdministrativeStatus!="NON_ADMIN"{fail(fmt.Sprintf("valid BFE Drive identity: %+v err=%v",caller,e))}
 case "invalid":if e!=authentication.ErrAuthInvalid||caller!=(authentication.Caller{}){fail(fmt.Sprintf("revoked token: %+v err=%v",caller,e))}
 case "unavailable":if e!=authentication.ErrAuthUnavailable||caller!=(authentication.Caller{}){fail(fmt.Sprintf("missing UDS: %+v err=%v",caller,e))}
 default:fail("invalid --expect")
 }
 fmt.Printf("real BFE Drive UDS: %s PASS\n",*expect)
}
func must(err error){if err!=nil{fail(err.Error())}}
func fail(s string){fmt.Fprintln(os.Stderr,s);os.Exit(1)}
