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
 private:=fixturePrivateKey()
 if *expect=="jkt"{fmt.Print(thumbprint(private));return}
 proof:=signedDPoP(private,*token)
 validator,err:=authentication.NewValidator(*socket,5*time.Minute)
 must(err)
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
 defer cancel()
 caller,authErr:=validator.ValidateCaller(ctx,*token,proof,authentication.RequestBinding{Method:"GET",Target:"https://chat.example.test/connection"})
 assertExpected(*expect,caller,authErr)
 fmt.Printf("real BFE Drive UDS: %s PASS\n",*expect)
}
func fixturePrivateKey()ed25519.PrivateKey{
 seed:=sha256.Sum256([]byte("dp-m01-independent-fixture-private-key"))
 return ed25519.NewKeyFromSeed(seed[:])
}
func thumbprint(private ed25519.PrivateKey)string{
 pub:=private.Public().(ed25519.PublicKey)
 thumb,err:=(&jose.JSONWebKey{Key:pub}).Thumbprint(crypto.SHA256)
 must(err)
 return base64.RawURLEncoding.EncodeToString(thumb)
}
func signedDPoP(private ed25519.PrivateKey,token string)string{
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:private,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
 must(err)
 ath:=sha256.Sum256([]byte(token))
 claims,err:=json.Marshal(map[string]any{"jti":fmt.Sprintf("qa-%d",time.Now().UnixNano()),"iat":time.Now().Unix(),"htm":"GET","htu":"https://chat.example.test/connection","ath":base64.RawURLEncoding.EncodeToString(ath[:])})
 must(err)
 signed,err:=signer.Sign(claims)
 must(err)
 proof,err:=signed.CompactSerialize()
 must(err)
 return proof
}
func assertExpected(expect string,caller authentication.Caller,err error){
 switch expect{
 case "ok":
  if err!=nil||caller.AccountID==""||caller.AdministrativeStatus!="NON_ADMIN"{fail(fmt.Sprintf("valid BFE Drive identity: %+v err=%v",caller,err))}
 case "invalid":
  if err!=authentication.ErrAuthInvalid||caller!=(authentication.Caller{}){fail(fmt.Sprintf("revoked token: %+v err=%v",caller,err))}
 case "unavailable":
  if err!=authentication.ErrAuthUnavailable||caller!=(authentication.Caller{}){fail(fmt.Sprintf("missing UDS: %+v err=%v",caller,err))}
 default:fail("invalid --expect")
 }
}
func must(err error){if err!=nil{fail(err.Error())}}
func fail(s string){fmt.Fprintln(os.Stderr,s);os.Exit(1)}
