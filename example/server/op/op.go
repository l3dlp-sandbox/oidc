package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/text/language"

	"github.com/caos/oidc/v2/example/server/internal"
	"github.com/caos/oidc/v2/pkg/op"
)

const (
	pathLoggedOut = "/logged-out"
)

func init() {
	internal.RegisterClients(
		internal.NativeClient("native"),
		internal.WebClient("web", "secret"),
		internal.WebClient("api", "secret"),
	)
}

func main() {
	ctx := context.Background()

	//we will run on :9998
	port := "9998"
	//which gives us the issuer: //http://localhost:9998/
	issuer := fmt.Sprintf("http://localhost:%s/", port)

	//the OpenID Provider requires a 32-byte key for (token) encryption
	//be sure to create a proper crypto random key and manage it securely!
	key := sha256.Sum256([]byte("test"))

	router := mux.NewRouter()

	//for simplicity, we provide a very small default page for users who have signed out
	router.HandleFunc(pathLoggedOut, func(w http.ResponseWriter, req *http.Request) {
		_, err := w.Write([]byte("signed out successfully"))
		if err != nil {
			log.Printf("error serving logged out page: %v", err)
		}
	})

	//the OpenIDProvider interface needs a Storage interface handling various checks and state manipulations
	//this might be the layer for accessing your database
	//in this example it will be handled in-memory
	storage := internal.NewStorage(issuer)

	//creation of the OpenIDProvider with the just created in-memory Storage
	provider, err := newOP(ctx, storage, issuer, key)
	if err != nil {
		log.Fatal(err)
	}

	//the provider will only take care of the OpenID Protocol, so there must be some sort of UI for the login process
	//for the simplicity of the example this means a simple page with username and password field
	l := NewLogin(storage, op.AuthCallbackURL(provider))

	//regardless of how many pages / steps there are in the process, the UI must be registered in the router,
	//so we will direct all calls to /login to the login UI
	router.PathPrefix("/login/").Handler(http.StripPrefix("/login", l.router))

	//we register the http handler of the OP on the root, so that the discovery endpoint (/.well-known/openid-configuration)
	//is served on the correct path
	//
	//if your issuer ends with a path (e.g. http://localhost:9998/custom/path/),
	//then you would have to set the path prefix (/custom/path/):
	//router.PathPrefix("/custom/path/").Handler(http.StripPrefix("/custom/path", provider.HttpHandler()))
	router.PathPrefix("/").Handler(provider.HttpHandler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	<-ctx.Done()
}

//newOP will create an OpenID Provider for localhost on a specified port with a given encryption key
//and a predefined default logout uri
//it will enable all options (see descriptions)
func newOP(ctx context.Context, storage op.Storage, issuer string, key [32]byte) (*op.Provider, error) {
	config := &op.Config{
		CryptoKey: key,

		//will be used if the end_session endpoint is called without a post_logout_redirect_uri
		DefaultLogoutRedirectURI: pathLoggedOut,

		//enables code_challenge_method S256 for PKCE (and therefore PKCE in general)
		CodeMethodS256: true,

		//enables additional client_id/client_secret authentication by form post (not only HTTP Basic Auth)
		AuthMethodPost: true,

		//enables additional authentication by using private_key_jwt
		AuthMethodPrivateKeyJWT: true,

		//enables refresh_token grant use
		GrantTypeRefreshToken: true,

		//enables use of the `request` Object parameter
		RequestObjectSupported: true,

		//this example has only static texts (in English), so we'll set the here accordingly
		SupportedUILocales: []language.Tag{language.English},
	}
	handler, err := op.NewOpenIDProvider(ctx, issuer, config, storage,
		//we must explicitly allow the use of the http issuer
		op.WithAllowInsecure(),
		//as an example on how to customize an endpoint this will change the authorization_endpoint from /authorize to /auth
		op.WithCustomAuthEndpoint(op.NewEndpoint("auth")),
	)
	if err != nil {
		return nil, err
	}
	return handler, nil
}