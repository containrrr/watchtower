import { ChangeEvent, FormEvent, useState } from "react";
import { logIn } from "../services/Api";
import logo from "../assets/logo.png";

interface LoginProps {
    onLogin: () => void;
}

const Login = (props: LoginProps) => {
    const [password, setPassword] = useState("");
    const [remember, setRemember] = useState(false);
    const [error, setError] = useState("");

    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        if (event.target.type === "checkbox") {
            setRemember(event.target.checked);
        } else {
            setPassword(event.target.value);
        }
    };

    const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
        setError("");
        event.preventDefault();

        if (password === "") {
            return;
        }

        const loggedIn = await logIn(password, remember);
        if (loggedIn) {
            props.onLogin();
        } else {
            setError("Invalid password.");
        }
    };

    return (
        <div className="d-flex flex-column min-vh-100 justify-content-center align-items-center">
            <form className="form-signin text-center" style={{ width: 350 }} onSubmit={handleSubmit}>
                <img className="mb-4" src={logo} alt="Watchtower" width="200" height="200" />
                <h1 className="h3 mb-3 fw-normal">Please log in</h1>

                <div className="form-floating mb-3">
                    <input type="password" value={password} onChange={handleChange} className={"form-control" + (error ? " is-invalid" : "")} id="floatingPassword" placeholder="Password" required />
                    <label htmlFor="floatingPassword" className="user-select-none">Password</label>
                    {error &&
                        <div className="invalid-feedback">
                            {error}
                        </div>
                    }
                </div>
                <button className="w-100 btn btn-lg btn-primary mb-3" type="submit">Log in</button>

                <div className="checkbox mb-3">
                    <label>
                        <input type="checkbox" value="remember-me" checked={remember} onChange={handleChange} /> Remember me
                    </label>
                </div>

                <p className="mt-5 small">
                    <a href="https://containrrr.dev/watchtower/" className="text-muted small" title="Visit Watchtower" target="_blank">Powered by Watchtower</a>
                </p>
            </form>
        </div>
    );
};

export default Login;