import { useEffect, useState } from "react";
import ContainerView from "./components/ContainerView";
import Header from "./components/Header";
import Login from "./components/Login";
import Spinner from "./components/Spinner";
import { checkLogin, logOut } from "./services/Api";

const App = () => {
    const [loading, setLoading] = useState(true);
    const [loggedIn, setLoggedIn] = useState(false);

    useEffect(() => {
        setLoading(true);
        (async () => {
            setLoggedIn(await checkLogin());
            setLoading(false);
        })();
    }, []);

    const onLogIn = () => {
        setLoading(false);
        setLoggedIn(true);
    };

    const onLogOut = () => {
        logOut();
        setLoading(false);
        setLoggedIn(false);
    };

    if (loading === true) {
        return <div className="mt-5 pt-5"><Spinner /></div>;
    }

    if (loggedIn !== true) {
        return <Login onLogin={onLogIn} />;
    }

    return (
        <div>
            <Header onLogOut={onLogOut} />
            <ContainerView />
        </div>
    );
};

export default App;